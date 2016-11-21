window.onload = function () {
    var conn;
    var myClientId;
    var tanks = {};
    var bullets = {};
    var mapBitmap;
    var newMapBitmap;
    var newMapBitmapFadeInIntervalID;
    var utf8decoder = new TextDecoder("utf-8");

    var WIDTH = 1260;
    var HEIGHT = 620;

    var NEWMAP_TIMEOUT = 30000;

    var MSG_IN_MAP = 0
    var MSG_IN_STATE = 1
    var MSG_IN_GREETING = 2
    var MSG_IN_BULLET_STATE = 3
    var MSG_IN_DEATH = 4
    var MSG_IN_LEADERBOARD = 5
    var MSG_IN_MAP_CHANGE = 6

    var MSG_OUT_MOVING = 0
    var MSG_OUT_SHOOTING = 1
    var MSG_OUT_START = 32
    var GROUP = Math.round(Math.random()*100) % 2;

    var mapSet = false;
    var spaceMode = false;

    var inputState = {
      moving: 0,
      power: 0,
      direction: 1
    };

    render.init(WIDTH, HEIGHT);

    window.me = function() {
        return tanks[myClientId];
    }

    window.sendStart = function (login) {
        conn.send(String.fromCharCode(MSG_OUT_START) + login);
    }

    window.showLogin = function () {
        document.getElementById("overlay").style.display = "block";
        document.getElementById("login").style.display = "block";
    }

    function Tank (x, y, hp, angle, clientId) {
        this.x = x;
        this.y = y;
        this.gunAngle = angle;
        this.clientId = clientId;
        this.maxHp = hp;
        this.hp = this.maxHp;
        this.shape = new createjs.Container();
        this.shape.regX = 0;
        this.shape.regY = 25;
        this.shape.x = this.x;
        this.shape.y = this.y;
        this.gun = new createjs.Bitmap("/tank6-gun-fix1.png");
        this.body = new createjs.Bitmap("/tank6-body.png");
        this.hpLabel = new createjs.Text(this.hp, "11px Roboto", "Red");
        this.nickLabel = new createjs.Text("Tank" + clientId, "11px Roboto", "Red");
        this.nickLabel.textAlign = "center";
        this.nickLabel.maxWidth = 100;
        this.shape.addChild(this.gun);
        this.shape.addChild(this.body);
        this.shape.addChild(this.hpLabel);
        this.gun.regX = 46;
        this.gun.regY = 25;
        this.gun.rotation = angle;
        this.body.regX = 47;
        this.body.regY = 25;
        this.hpLabel.regX = 10;
        this.hpLabel.regY = -35;
        // TODO: Text-align center 
        this.nickLabel.regY = 25;
        this.shape.addChild(this.nickLabel);
        render.stage.addChild(this.shape);
    }

    Tank.prototype.updateState = function(x, y, hp, angle) {
        this.x = x;
        this.y = y;
        this.hp = hp;
        this.shape.x = this.x;
        this.shape.y = this.y;
        this.hpLabel.text = this.hp;
        if (!me() || me().clientId != this.clientId) {
            this.gunAngle = angle;
            this.gun.rotation = angle;
        }
    }

    Tank.prototype.updateName = function (name) {
        this.nickLabel.text = name;
    }

    Tank.prototype.destroy = function() {
        render.stage.removeChild(this.shape);
        render.stage.removeChild(this.hpLabel);
    }

    function Bullet (x, y, id) {
        this.x = x;
        this.y = y;
        this.id = id;
        this.shape = new createjs.Shape();
        this.shape.graphics.beginFill("Red").drawCircle(0, 0, 5);
        this.shape.x = this.x;
        this.shape.y = this.y;
        render.stage.addChild(this.shape);
    }

    Bullet.prototype.updateState = function(x, y) {
        this.x = x;
        this.y = y;
        this.shape.x = this.x;
        this.shape.y = this.y;
    }

    var MAX_LEADERS = 10;
    function LeaderboardEntry(id, name, frags) {
        this.id = id;
        this.name = name;
        this.frags = frags;
    }
    LeaderboardEntry.compare = function (x, y) {
        var dfrag = y.frags - x.frags;
        if (dfrag) return dfrag;
        return y.id - x.id;
    }

    var leaderboardLines = [];
    for (var i = 0; i < MAX_LEADERS; i++) {
        var line = new createjs.Text("", "12px Roboto", "Black");
        line.y = 5 + i * 14;
        line.x = 10;
        render.stage.addChild(line);
        leaderboardLines[i] = line;
    }
    function updateLeaderboard(entries) {
        entries.sort(LeaderboardEntry.compare);
        var numLeaders = 0;
        for (var i = 0; i < Math.min(entries.length, MAX_LEADERS); i++) {
            var leader = entries[i];
            if (leader.frags != 0) {
                numLeaders++;
                leaderboardLines[i].text = (i + 1) + ". " + leader.name + "\t" + leader.frags;
            }
        }
        for (var i = numLeaders; i < entries.length; i++) {
            leaderboardLines[i].text = "";
        }
    }

    function dearchive(mapBitmapArchive) {
       var mapBitmapBuffer = new ArrayBuffer(WIDTH * HEIGHT);
       var mapBitmap = new Int8Array(mapBitmapBuffer)
       var origCounter = 0, resCounter = 0;
       while (origCounter < mapBitmapArchive.byteLength) {
           var val = mapBitmapArchive.getUint8(origCounter);
           var num = mapBitmapArchive.getUint32(origCounter + 1, true);
           origCounter += 5;
           for (var k = 0; k < num; k++) {
               mapBitmap[resCounter] = val;
               resCounter++;
           }
       }
       return mapBitmap;
    }

    if (window["WebSocket"]) {
        conn = new WebSocket("ws://" + window.location.host + "/ws");
        conn.binaryType = "arraybuffer";
        conn.onclose = function (evt) {
            console.log("Connection closed")
        };
        conn.onerror = function (evt) {
            console.log("Connection failed, trying again");
            console.log(evt);
            conn = new WebSocket("ws://" + window.location.host + "/ws");
            conn.binaryType = "arraybuffer";
        };
        conn.onmessage = function (evt) {
            var dataView = new DataView(evt.data);
            // TODO: Replace with DataView
            switch (dataView.getUint8(0)) {
            case MSG_IN_MAP: // map update
                if (mapSet) {
                    newMapBitmap = new DataView(evt.data, 1); 
                    newMapBitmap = dearchive(newMapBitmap);
                    render.updateMapCanvasBack(newMapBitmap);
                    var newMapTimeStart = performance.now();
                    newMapBitmapFadeInIntervalID = setInterval(function () {
                        var elapsed = performance.now() - newMapTimeStart;
                        var percentDone = elapsed/(NEWMAP_TIMEOUT * 1.7);
                        render.bgImageBack.alpha = percentDone;
                    }, 50);
                } else {
                    mapSet = true;
                    mapBitmap = new DataView(evt.data, 1); 
                    console.log("1", mapBitmap);
                    mapBitmap = dearchive(mapBitmap);
                    console.log("2", mapBitmap);
                    render.updateMapCanvas(mapBitmap);
                }
                break;
            case MSG_IN_MAP_CHANGE:
                if (messageByteView[1] == 1) {
                    spaceMode = true;
                    for (var id in bullets) {
                        render.stage.removeChild(bullets[id].shape);
                    }
                    bullets = {};
                    function slideDownOldMap() {
                        if (spaceMode) {
                            render.bgImage.alpha -= 0.1;
                            setTimeout(slideDownOldMap, 100);
                        }
                    }
                    setTimeout(slideDownOldMap, 1);
                } else {
                    spaceMode = false;
                    mapBitmap = newMapBitmap;
                    if (newMapBitmapFadeInIntervalID) {
                        newMapBitmapFadeInIntervalID = clearInterval(
                            newMapBitmapFadeInIntervalID);
                    }
                    render.swapBgImage();
                }
                break;
            case MSG_IN_STATE:
                var clientsNum = messageByteView[1];
                var messageView = new Int32Array(evt.data.slice(2));
                for (var i = 0; i < clientsNum; i++) {
                    var clientId = messageView[i*5];
                    var tankClientX = messageView[i*5 + 1];
                    var tankClientY = messageView[i*5 + 2];
                    var tankClientHp = messageView[i*5 + 3];
                    var tankGunAngle = messageView[i*5 + 4];
                    if (!tanks[clientId]) {
                        tanks[clientId] =
                            new Tank(tankClientX,
                                tankClientY,
                                tankClientHp,
                                tankGunAngle,
                                clientId);
                    } else {
                        tanks[clientId].updateState(tankClientX,
                            tankClientY,
                            tankClientHp,
                            tankGunAngle);
                    }
                }
                break;
            case MSG_IN_LEADERBOARD:
                var clientsNum = dataView.getUint32(1, true);
                var leaderboard = new Array(clientsNum);
                var p = 5;
                for (var i = 0; i < clientsNum; i++) {
                    var id = dataView.getUint32(p, true);
                    var sessionFrags = dataView.getUint32(p + 4, true);
                    var lifeFrags = dataView.getUint32(p + 8, true);
                    var nameLen = dataView.getUint8(p + 12);
                    var nameView = new DataView(evt.data, p + 13, nameLen);
                    var name = utf8decoder.decode(nameView);
                    if (tanks[id]) {
                        tanks[id].updateName(name);
                    }
                    var frags = GROUP == 0 ? lifeFrags : sessionFrags;
                    leaderboard[i] = new LeaderboardEntry(id, name, frags);
                    p += 13 + nameLen;
                }
                updateLeaderboard(leaderboard);
                break;
            case MSG_IN_GREETING:
                myClientId = (new Int32Array(evt.data.slice(1)))[0];
                console.log("I am " + myClientId);
                break;
            case MSG_IN_BULLET_STATE:
                if (!spaceMode) {
                    var bulletsNum = dataView.getUint32(1, true);
                    var headerOffset = 5;
                    for (var i = 0; i < bulletsNum; i++) {
                        var id = dataView.getUint32(headerOffset + i*12, true);
                        var bulletX = dataView.getUint32(headerOffset + i*12 + 4, true);
                        var bulletY = dataView.getUint32(headerOffset + i*12 + 8, true);
                        if (!bullets[id]) {
                            bullets[id] =
                                new Bullet(bulletX, bulletY, id);
                        } else {
                            bullets[id].updateState(bulletX, bulletY);
                        }
                    }
                }
                break;
            case MSG_IN_DEATH:
                var messageData = new Int32Array(evt.data.slice(1));
                var deadId = messageData[0],
                    x = messageData[1],
                    y = messageData[2],
                    radius = messageData[3];
                // console.log(deadId + " died at " + x + ", " + y + " with r=" + radius);
                if (radius > 0 && !spaceMode) {
                    if (!mapBitmap) {
                        // TODO: discover why?!
                        console.log("BUG!")
                    } else {
                        explodeAt(mapBitmap, x, y, radius);
                    }
                }
                if (tanks.hasOwnProperty(deadId)) {
                    tanks[deadId].destroy();
                    if (deadId == myClientId) {
                        inputState.direction = 1;
                        inputState.power = 0;
                        showLogin();
                    }
                    delete tanks[deadId];
                } else if (bullets.hasOwnProperty(deadId)) {
                    var bullet = bullets[deadId];
                    var explosion = new createjs.Bitmap("/explosion5.png");
                    explosion.scaleX = 0.1;
                    explosion.scaleY = 0.1;
                    explosion.x = x;
                    explosion.y = y;
                    explosion.rotation = Math.random() * 360;
                    render.stage.addChild(explosion);
                    explosion.regX = 64;
                    explosion.regY = 64;
                    var times = 3;
                    function animateExplosion() {
                        if (times > 0) {
                            explosion.scaleX += 0.2;
                            explosion.scaleY += 0.2;
                            setTimeout(animateExplosion, 100);
                            times--;
                        } else {
                            render.stage.removeChild(explosion);
                        }
                    }
                    animateExplosion();
                    render.stage.removeChild(bullet.shape);
                    delete bullets[deadId];
                }
            }
        };
    } else {
        var item = document.createElement("div");
        body.innerHTML = "<b>Your browser does not support WebSockets.</b>";
        return;
    }

    function changeAngle(arrowUp) {
        var cosAngle = Math.cos(me().gun.rotation * Math.PI / 180);
        var sinAngle = Math.sin(me().gun.rotation * Math.PI / 180);
        if (!arrowUp && sinAngle > 0 && Math.abs(cosAngle) < 0.92) {
            return; // gun is already too low
        }
        if (arrowUp && sinAngle < 0 && Math.abs(cosAngle) < 0.09) {
            return; // gun is already too high
        }
        var inc = 2 * cosAngle / Math.abs(cosAngle);
        me().gun.rotation += arrowUp ? -1 * inc : inc;
    }

    function getShootingPower(inputState) {
        var power = Math.floor((Date.now() - inputState.power) / 200);
        if (power <= 0) {
            power = 1;
        }
        if (power > 10) {
            power = 10;
        }
        return power;
    }


    window.addEventListener("keydown", function(e) {
        if (e.code == "Enter" &&
            document.getElementById("overlay").style.display != "none") {
            onPlayButtonClicked();
            return;
        }
        if (!me()) {
            return;
        }
        if (e.code == "ArrowLeft" || e.code == "ArrowRight") {
            var newDir = (e.code == "ArrowLeft") ? -1 : 1;
            if (inputState.direction != newDir) {
                me().gun.rotation = 180 - me().gun.rotation;
            }
            inputState.direction = newDir;
            inputState.moving = newDir;
        }
        if (e.code == "ArrowUp" || e.code == "ArrowDown") {
            changeAngle(e.code == "ArrowUp");
        }
        if (e.code == "Space") {
            if (inputState.power == 0) {
                inputState.power = Date.now();
                function updateGun() {
                    if (inputState.power == 0) {
                        return;
                    }
                    var power = getShootingPower(inputState);
                    me().gun.filters = [new createjs.ColorFilter(1, 1, 1, 1, power/10*150, 0, 0, 0)];
                    me().gun.cache(0, 0, 100, 100);
                    setTimeout(updateGun, 100);
                }
                updateGun();
            }
        }
    });

    window.addEventListener("keyup", function(e) {
        if (!me()) {
            return;
        }
        if (e.code == "ArrowLeft" || e.code == "ArrowRight") {
          inputState.moving = 0;
        }
        if (e.code == "Space") {
            inputState.power = getShootingPower(inputState);
            var a = me().gun.rotation * Math.PI / 180;
            var aimX = Math.cos(a) * 1000,
                aimY = Math.sin(a) * 1000;
            var messageBuffer = new ArrayBuffer(10);
            var dataView = new DataView(messageBuffer);
            dataView.setUint8(0, MSG_OUT_SHOOTING);
            dataView.setInt32(1, aimX, true /* little endian */);
            dataView.setInt32(5, aimY, true /* little endian */);
            dataView.setUint8(9, inputState.power);
            conn.send(messageBuffer);
            inputState.power = 0;
            me().gun.filters = [];
            me().gun.cache(0, 0, 100, 100);
        }
    });

    setInterval(function() { // Sending packets to server
          var messageBuffer = new ArrayBuffer(6);
          var dataView = new DataView(messageBuffer);
          dataView.setUint8(0, MSG_OUT_MOVING);
          dataView.setUint8(1, inputState.moving);
          var rotation = 0;
          if (me()) {
              rotation = me().gun.rotation;
          }
          dataView.setInt32(2, rotation, true /* Little endian */);
          if (conn.readyState === conn.OPEN) {
              conn.send(dataView);
          } else {
              //console.log("Still waiting for the conn to be established");
          }
    }, 200);

    function explodeAt(mapBitmap, cx, cy, r) {
        var rs = (r*r)|0;
        var sx = Math.max(cx - r, 0)|0, sy = Math.max(cy - r, 0)|0;
        var ly = Math.min(cy + r, HEIGHT)|0, lx = Math.min(cx + r, WIDTH)|0;
        for (var y = sy; y < ly; y++) {
            for (var x = sx; x < lx; x++) {
                var ds = Math.pow(y-cy, 2) + Math.pow(x-cx, 2);
                if (ds < rs) {
                    mapBitmap[(x|0) + (y|0)*WIDTH] = 0;
                }
            }
        }
        render.updateMapCanvasPartial(mapBitmap, sx, sy, 2*r, 2*r);
    }

    var fpsLabel = new createjs.Text("60", "9px Roboto", "Red");
    fpsLabel.textAlign = "end";
    fpsLabel.x = 1260 - 1;
    render.stage.addChild(fpsLabel);

    // Render loop.
    var startTime = performance.now();
    var i = 0;
    var deltas = [];
    createjs.Ticker.on("tick", tick);
    createjs.Ticker.setFPS(60);
    function tick(evt) {
        render.redraw();

        var endTime = performance.now();
        var deltaTime = endTime - startTime;
        startTime = endTime;
        fpsLabel.text = (1000/average(deltas))|0;
        deltas[i%8] = deltaTime;
        i++;
    }

    function average(xs) {
        var s = 0;
        for (var i = 0; i < xs.length; ++i) {
            s += xs[i];
        }
        return s/xs.length;
    }
};

function onPlayButtonClicked () {
    var login = document.getElementById("loginInput").value;
    document.getElementById("overlay").style.display = "none";
    document.getElementById("login").style.display = "none";
    sendStart(login);
}
