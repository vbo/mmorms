(function () {
    var render = window.render = {};

    render.init = function (width, height) {
        this.width = width;
        this.height = height;
        this.stage = new createjs.Stage("gameCanvas");
        this.canvas = document.getElementById('maskCanvas');
        this.canvasBack = document.getElementById('maskCanvasBack');
        this.ctx = this.canvas.getContext('2d');
        this.ctxBack = this.canvasBack.getContext('2d');
        this.maskImageData = this.ctx.createImageData(width, height);
        this.maskImageDataBack = this.ctxBack.createImageData(width, height);
        var bgSky = new createjs.Shape();
        bgSky.graphics.beginFill("lightcyan").drawRect(0, 0, width, height);
        this.stage.addChild(bgSky);

        // Background "theme" image.
        this.bgImage = new createjs.Bitmap("/theme.jpg");
        this.bgImageFilter = new createjs.AlphaMaskFilter(this.canvas);
        this.bgImage.filters = [this.bgImageFilter]
        this.bgImageBack = new createjs.Bitmap("/theme.jpg");
        this.bgImageBackFilter = new createjs.AlphaMaskFilter(this.canvasBack);
        this.bgImageBack.filters = [this.bgImageBackFilter]
        this.bgImageBack.alpha = 0;
        this.stage.addChild(this.bgImage);
        this.stage.addChild(this.bgImageBack);
    }

    render.swap = function(a, b) {
        var tmp = render[a];
        render[a] = render[b];
        render[b] = tmp;
    }

    render.swapBgImage = function () {
        render.swap("ctx", "ctxBack");
        render.swap("maskImageData", "maskImageDataBack");
        render.swap("canvas", "canvasBack");
        this.bgImageBack.alpha = 0;
        this.bgImage.alpha = 1;
        this.bgImageFilter.mask = this.canvas;
        this.bgImageBackFilter.mask = this.canvasBack;
        this.bgImage.y = 0;
        this.bgImageBack.y = 0;
    }

    // Draw the map bitmap to a canvas to use
    // it as an alpha mask for map theme image

    // Update pixel data and blit back to canvas (~5ms cold)
    render.updateMapCanvas = function (mapBitmap) {
        updateMaskPixelsFromMap(mapBitmap, this.maskImageData.data);
        this.ctx.putImageData(this.maskImageData, 0, 0);
    }

    render.updateMapCanvasBack = function (mapBitmap) {
        updateMaskPixelsFromMap(mapBitmap, this.maskImageDataBack.data);
        this.ctxBack.putImageData(this.maskImageDataBack, 0, 0);
    }

    render.updateMapCanvasPartial = function (mapBitmap, x, y, w, h) {
        updateMaskPixelsFromMap(mapBitmap, this.maskImageData.data);
        this.ctx.putImageData(this.maskImageData, 0, 0, x, y, w, h);
    }

    render.redraw = function () {
        this.bgImage.cache(0, 0, this.width, this.height); // every frame?
        this.bgImageBack.cache(0, 0, this.width, this.height); // every frame?
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
