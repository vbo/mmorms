(function () {
    var render = window.render = {};

    render.init = function (width, height) {
        this.width = width;
        this.height = height;
        this.stage = new createjs.Stage("gameCanvas");
        var canvas = document.getElementById('maskCanvas');
        this.ctx = canvas.getContext('2d');
        this.maskImageData = this.ctx.createImageData(width, height);
        var bgSky = new createjs.Shape();
        bgSky.graphics.beginFill("lightcyan").drawRect(0, 0, width, height);
        this.stage.addChild(bgSky);

        // Background "theme" image.
        this.bgImage = new createjs.Bitmap("/theme.jpg");
        this.bgImage.filters = [
            new createjs.AlphaMaskFilter(canvas)
        ];
        this.stage.addChild(this.bgImage);
    }

    // Draw the map bitmap to a canvas to use
    // it as an alpha mask for map theme image

    // Update pixel data and blit back to canvas (~5ms cold)
    render.updateMapCanvas = function (mapBitmap) {
        updateMaskPixelsFromMap(mapBitmap, this.maskImageData.data);
        this.ctx.putImageData(this.maskImageData, 0, 0);
    }

    render.updateMapCanvasPartial = function (mapBitmap, x, y, w, h) {
        updateMaskPixelsFromMap(mapBitmap, this.maskImageData.data);
        this.ctx.putImageData(this.maskImageData, 0, 0, x, y, w, h);
    }

    render.redraw = function () {
        this.bgImage.cache(0, 0, this.width, this.height); // every frame?
        this.stage.update();
    }

    function updateMaskPixelsFromMap(map, pixels) {
        for (var p = 0, l = map.length; p < l; p += 4) {
            var q = p*4;
            pixels[q+3] = map[p] * 255;
            pixels[q+7] = map[p+1] * 255;
            pixels[q+11] = map[p+2] * 255;
            pixels[q+15] = map[p+3] * 255;
        }
    }
})();
