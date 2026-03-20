window.onload = function () {
    "use strict";

    var HINTS = [
        'By holding the "c" button you shoot further.',
        'By holding the "x" button you jump higher.',
        '"z" button turns on the shield.',
        '"x" button allows you to jump.',
        'Make sure you have a good speed before jumping',
        'White color inside the tank means lack of energy for the shield.',
        'As you level up the power of your bullets increase.',
        'When you destroy a tank, you get all it\'s power.',
        'It\'s open-source! github.com/vbo/mmorms',
    ];

    var KEYCODE_ENTER = 13;
    var KEYCODE_Z = 90;
    var KEYCODE_X = 88;
    var KEYCODE_C = 67;
    var KEYCODE_UP = 38;
    var KEYCODE_RIGHT = 39;
    var KEYCODE_DOWN = 40;
    var KEYCODE_LEFT = 37;

    var WIDTH = 1260;
    var HEIGHT = 620;

    var NEWMAP_TIMEOUT = 30000;
    var GRAV_ACC = 30; 

    var MSG_IN_MAP = 0;
    var MSG_IN_STATE = 1;
    var MSG_IN_GREETING = 2;
    var MSG_IN_DEATH = 4;
    var MSG_IN_LEADERBOARD = 5;
    var MSG_IN_MAP_CHANGE = 6;
    var MSG_IN_BULLET_CREATE = 7;

    var MSG_OUT_MOVING = 0;
    var MSG_OUT_SHOOTING = 1;
    var MSG_OUT_SHIELD = 2;
    var MSG_OUT_JUMP = 3;
    var MSG_OUT_START = 32;

    var JUMP_POWERUP_SPEED = 0.004;
    var SHOOT_POWERUP_SPEED = 0.0075;

    var INTERPOLATION_ENABLED = true;

    var utf8decoder = new TextDecoder("utf-8");

    var conn;

    // NOTE(vbo): global, so we can call it from onclick
    window.sendStart = function (login) {
        if (!conn || conn.readyState !== conn.OPEN) {
            setTimeout(function() {
                window.sendStart(login);
            }, 100);
        } else {
            conn.send(String.fromCharCode(MSG_OUT_START) + login);
        }
    }

    function showLogin(frags) {
        document.getElementById("overlay").style.display = "block";
        document.getElementById("login").style.display = "block";
        if (typeof frags !== "undefined") {
            document.getElementById("login").style.height = "256px";
            document.getElementById("linkbar").style.marginTop = "40px";
            document.getElementById("finalScoreBlock").style.display = "block";
            document.getElementById("lifeScore").textContent = frags;
        }
        var index = Math.floor(Math.random() * HINTS.length);
        document.getElementById("randomHint").textContent = 
            HINTS[index];
    }
    showLogin();

    var myClientId;
    var tanks = {};
    var bullets = {};
    var mapBitmap;
    var newMapBitmap;
    var newMapBitmapFadeInIntervalID;

    var mapSet = false;
    var spaceMode = false;
    var firstMessageServerTime = -1;
    var firstMessageClientTime = -1;

    var inputState = {
      moving: 0,
      startChangingAngle: 0,
      changingAngle: 0,
      wasMoving: 0,
      wasChangingAngle: 0,
      jumpPower: 0,
      shootPower: 0
    };

    render.init(WIDTH, HEIGHT);

    window.playButtonClicked = 0;
    window.me = function() {
        return tanks[myClientId];
    }

    function Tank (x, y, hp, angle, shield, shieldPercent, direction, clientId) {
        this.x = x;
        this.y = y;
        this.clientId = clientId;
        this.maxHp = hp;
        this.hp = this.maxHp;
        this.direction = direction;
        this.shape = new createjs.Container();
        this.shape.regX = 0;
        this.shape.regY = 25;
        this.shape.x = this.x;
        this.shape.y = this.y;
        this.gun = new createjs.Bitmap("/tank6-gun-fix1.png");
        this.body = new createjs.Bitmap("/tank6-body-fix1.png");

        this.hpLabel = new createjs.Text(this.hp, "11px Roboto", "Red");
        this.hpLabel.textAlign = "center";
        
        this.shield = new createjs.Shape();
        this.shield.graphics.beginStroke("blue").beginFill("blue").drawCircle(0, 0, 35);
        this.shield.alpha = 0.1;
        this.shield.visible = !!shield;

        this.shieldBar = new createjs.Shape();
        this.shieldBar.graphics.beginStroke("white").drawRect(-3, -1, 7, 2);
        this.shieldBar.scaleX = shield ? shieldPercent : (1 - shieldPercent);
        this.shieldBar.shadow = new createjs.Shadow("white", 0, 0, 2);

        this.shootBar = new createjs.Shape();
        this.shootBar.graphics.setStrokeStyle(2).beginStroke("red")
            .drawRect(-18, 16, 37, 4);
        this.shootBar.alpha = 0.9;
        this.shootBar.visible = false;

        this.shootProgress = new createjs.Shape();
        this.shootProgress.graphics.setStrokeStyle(2).beginStroke("red")
            .drawRect(-18, 17, 37, 2);
        this.shootProgress.alpha = 0.8;
        this.shootProgress.visible = false;

        this.jumpBar = new createjs.Shape();
        this.jumpBar.graphics.setStrokeStyle(2).beginStroke("white")
            .drawRect(-18, 22, 37, 4);
        this.jumpBar.alpha = 0.8;
        this.jumpBar.visible = false;

        this.jumpProgress = new createjs.Shape();
        this.jumpProgress.graphics.setStrokeStyle(2).beginStroke("white")
            .drawRect(-18, 23, 37, 2);
        this.jumpProgress.alpha = 0.7;
        this.jumpProgress.visible = false;

        this.nickLabel = new createjs.Text("Tank" + clientId, "11px Roboto", "Red");
        this.nickLabel.textAlign = "center";
        this.nickLabel.maxWidth = 100;
        
        this.shape.addChild(this.gun);
        this.shape.addChild(this.body);
        this.shape.addChild(this.hpLabel);
        this.shape.addChild(this.shield);
        this.shape.addChild(this.shieldBar);
        this.shape.addChild(this.nickLabel);
        this.shape.addChild(this.jumpBar);
        this.shape.addChild(this.jumpProgress);
        this.shape.addChild(this.shootBar);
        this.shape.addChild(this.shootProgress);
        
        this.gun.regX = 46;
        this.gun.regY = 25;
        this.gun.rotation = angle;
        this.body.regX = 47;
        this.body.regY = 25;
        this.hpLabel.regY = -35;
        this.shield.regY = -10;
        this.shieldBar.regY = 0;
        // TODO: Text-align center 
        this.nickLabel.regY = 25;
        this.lifeFrags = 0;
        render.stage.addChild(this.shape);
    }

    Tank.prototype.updateState = function(x, y, hp, angle, shield, shieldPercent, direction) {
        this.x = x;
        this.y = y;
        if (me() && me().clientId == this.clientId && hp < this.hp) {
            var hurtScreen = new createjs.Shape();
            hurtScreen.graphics.beginFill("red").drawRect(0,0, WIDTH, HEIGHT);
            hurtScreen.alpha = 0.1;
            render.stage.addChild(hurtScreen);
            var pickReached = false;
            var animateHurtScreen = function() {
                if (!pickReached) {
                    hurtScreen.alpha += 0.1;
                    if (hurtScreen.alpha >= 0.3) {
                        pickReached = true;
                    }
                    setTimeout(animateHurtScreen, 30);
                } else {
                    hurtScreen.alpha -= 0.05;
                    if (hurtScreen.alpha > 0) {
                        setTimeout(animateHurtScreen, 30);
                    } else {
                        render.stage.removeChild(hurtScreen);
                    }
                }
            };
            setTimeout(animateHurtScreen, 10);
        }
        this.hp = hp;
        // TODO: very rarely bugs happen here. No idea why =/
        if (INTERPOLATION_ENABLED && me() && this.clientId != me().clientId) {
            this.direction = direction;
        }
        if (!INTERPOLATION_ENABLED || ! me() || this.clientId != me().clientId || direction == this.direction) {
            this.gun.rotation = angle;
        }
        this.shape.x = this.x;
        this.shape.y = this.y;
        this.hpLabel.text = this.hp;
        this.shield.visible = !!shield;
        this.shieldBar.scaleX = shield ? shieldPercent : (1 - shieldPercent);
    }

    Tank.prototype.updateName = function (name, lifeFrags) {
        this.lifeFrags = lifeFrags;
        if (lifeFrags > 0) {
            name += " ";
        }
        var required = 1;
        while (lifeFrags > 0) {
            if (lifeFrags >= required) {
                name += "★";
                lifeFrags -= required;
                required *= 2;
            } else {
                break;
            }
        }
        this.nickLabel.text = name;
    }

    Tank.prototype.destroy = function() {
        render.stage.removeChild(this.shape);
        render.stage.removeChild(this.hpLabel);
    }

    function Bullet (id, ownerId, x, y, vx, vy, creationTime) {
        this.x0 = x;
        this.y0 = y;
        this.vx = vx;
        this.vy = vy;
        this.id = id;
        this.ownerId = ownerId;
        this.creationTime = creationTime;
        this.shape = new createjs.Shape();
        this.shape.graphics.beginFill("Red").drawCircle(0, 0, 5);
        this.shape.x = this.x0;
        this.shape.y = this.y0;
        render.stage.addChild(this.shape);
    }
    
    Bullet.prototype.updateState = function(t) {
        var dt = (t - this.creationTime) / 1000;
        if (dt < 0) {
            return;
        }
        this.shape.x = (this.x0 + this.vx * dt) | 0;
        this.shape.y = (this.y0 + this.vy * dt + GRAV_ACC * dt * dt / 2) | 0;
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

    var leaderBoardTitle = new createjs.Text("LEADERBOARD", "12px Roboto", "Black");
    leaderBoardTitle.visible = false;
    leaderBoardTitle.y = 5;
    leaderBoardTitle.x = 45;
    render.stage.addChild(leaderBoardTitle);
    var leaderboardNickLines = [];
    var leaderboardFragLines = [];
    for (var i = 0; i <= MAX_LEADERS; i++) {
        var line = new createjs.Text("", "12px Roboto", "Black");
        var yOffset = 25 + i*14;
        var nickWidth = 130;
        line.y = yOffset;
        line.x = 10;
        line.lineWidth = nickWidth;
        render.stage.addChild(line);
        leaderboardNickLines[i] = line;

        line = new createjs.Text("", "12px Roboto", "Black");
        line.y = yOffset;
        line.x = nickWidth + 10;
        render.stage.addChild(line);
        leaderboardFragLines[i] = line;
    }

    function updateLeaderboard(entries) {
        entries.sort(LeaderboardEntry.compare);
        for (var i = 0; i < MAX_LEADERS; i++) {
            leaderboardNickLines[i].text = "";
            leaderboardFragLines[i].text = "";
        }
        leaderBoardTitle.visible = false;
        for (var i = 0; i <= Math.min(entries.length, MAX_LEADERS); i++) {
            var leader = entries[i];
            if (leader && leader.frags != 0) {
                leaderBoardTitle.visible = true;
                leaderboardNickLines[i].text = "#" + i + "    " + leader.name.substring(0,12);
                leaderboardFragLines[i].text = leader.frags;
            }
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

    var connectToGameServer = function (gameServerWSUrl) {
        conn = new WebSocket(gameServerWSUrl);
        conn.binaryType = "arraybuffer";
        conn.onclose = function (evt) {
            console.log("Connection closed")
        };
        conn.onerror = function (evt) {
            // TODO(vbo): handle disconnection gracefully
            console.log("Connection failed unexpectedly");
            console.log(evt);
        };
        conn.onmessage = function (evt) {
            var dataView = new DataView(evt.data);
            var messageByteView = new Uint8Array(evt.data);
            // TODO: Replace with DataView
            var type = dataView.getUint8(0);
            var msgServerTime = 0;
            var msgClientTime = 0;
            var headerSize = 1;
            if (type != MSG_IN_MAP)  {
                headerSize = 9;
                if (dataView.byteLength < 2) {
                    console.log(type);
                }
                msgServerTime = dataView.getFloat64(1, true);
                if (firstMessageServerTime < 0) {
                    firstMessageServerTime = msgServerTime;
                    firstMessageClientTime = performance.now();
                }
                msgClientTime = firstMessageClientTime + (msgServerTime - firstMessageServerTime);
            }
            switch (type) {
            case MSG_IN_MAP: // map update
                if (mapSet) {
                    newMapBitmap = new DataView(evt.data, headerSize); 
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
                    mapBitmap = new DataView(evt.data, headerSize); 
                    mapBitmap = dearchive(mapBitmap);
                    render.updateMapCanvas(mapBitmap);
                }
                break;
            case MSG_IN_MAP_CHANGE:
                if (messageByteView[headerSize] == 1) {
                    spaceMode = true;
                    for (var id in bullets) {
                        render.stage.removeChild(bullets[id].shape);
                    }
                    bullets = {};
                    var slideDownOldMap = function () {
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
                var clientsNum = messageByteView[headerSize];
                var numItemsPerTank = 7;
                var messageView = new Int32Array(evt.data.slice(headerSize + 1));
                for (var i = 0; i < clientsNum; i++) {
                    var clientId = messageView[i*numItemsPerTank];
                    var tankClientX = messageView[i*numItemsPerTank + 1];
                    var tankClientY = messageView[i*numItemsPerTank + 2];
                    var tankClientHp = messageView[i*numItemsPerTank + 3];
                    var tankGunAngle = messageView[i*numItemsPerTank + 4];
                    var shieldInfo = messageView[i*numItemsPerTank + 5];
                    var shield = shieldInfo >> 24;
                    var shieldPercent = (shieldInfo & 0xFF)/255;
                    var direction = messageView[i*numItemsPerTank + 6];
                    if (!tanks[clientId]) {
                        tanks[clientId] =
                            new Tank(tankClientX,
                                tankClientY,
                                tankClientHp,
                                tankGunAngle,
                                shield,
                                shieldPercent,
                                direction,
                                clientId);
                    } else {
                        tanks[clientId].updateState(
                            tankClientX,
                            tankClientY,
                            tankClientHp,
                            tankGunAngle,
                            shield,
                            shieldPercent,
                            direction);
                    }
                }
                break;
            case MSG_IN_LEADERBOARD:
                var clientsNum = dataView.getUint32(headerSize, true);
                var leaderboard = new Array(clientsNum);
                var p = headerSize + 4;
                for (var i = 0; i < clientsNum; i++) {
                    var id = dataView.getUint32(p, true);
                    var sessionFrags = dataView.getUint32(p + 4, true);
                    var lifeFrags = dataView.getUint32(p + 8, true);
                    var nameLen = dataView.getUint8(p + 12);
                    var nameView = new DataView(evt.data, p + 13, nameLen);
                    var name = utf8decoder.decode(nameView);
                    if (tanks[id]) {
                        tanks[id].updateName(name, lifeFrags);
                    }
                    leaderboard[i] = new LeaderboardEntry(id, name, lifeFrags);
                    p += 13 + nameLen;
                }
                updateLeaderboard(leaderboard);
                break;
            case MSG_IN_GREETING:
                myClientId = dataView.getUint32(headerSize, true);
                console.log("I am " + myClientId);
                break;
            case MSG_IN_BULLET_CREATE:
                var id = dataView.getUint32(headerSize, true);
                var ownerId = dataView.getUint32(headerSize + 4, true);
                var x = dataView.getFloat32(headerSize + 8, true);
                var y = dataView.getFloat32(headerSize + 12, true);
                var vx = dataView.getFloat32(headerSize + 16, true);
                var vy = dataView.getFloat32(headerSize + 20, true);
                bullets[id] = new Bullet(id, ownerId, x, y, vx, vy, msgClientTime);
                break;
            case MSG_IN_DEATH:
                var messageData = new Int32Array(evt.data.slice(headerSize));
                var deadId = messageData[0];
                var x = messageData[1];
                var y = messageData[2];
                var radius = messageData[3];
                // console.log(deadId + " died at " + x + ", " + y + " with r=" + radius);
                if (radius > 0 && !spaceMode) {
                    if (!mapBitmap) {
                        // TODO: discover why?!
                        console.log("BUG!")
                    } else {
                        explodeAt(mapBitmap, x, y, radius);
                        if (radius > 20) {
                            var explosion = new createjs.Bitmap("/explosion5.png");
                            explosion.scaleX = 0.1;
                            explosion.scaleY = 0.1;
                            explosion.x = x;
                            explosion.y = y;
                            explosion.rotation = Math.random() * 360;
                            render.stage.addChild(explosion);
                            explosion.regX = 64;
                            explosion.regY = 64;
                            var maxScale = radius / 64;
                            var animateExplosion = function () {
                                if (explosion.scaleX < maxScale) {
                                    explosion.scaleX += maxScale*0.4;
                                    explosion.scaleY += maxScale*0.4;
                                    if (explosion.scaleX > maxScale) {
                                        explosion.scaleX = maxScale;
                                        explosion.scaleY = maxScale;
                                    }
                                    setTimeout(animateExplosion, 100);
                                } else {
                                    render.stage.removeChild(explosion);
                                }
                            }
                            animateExplosion();
                        }
                    }
                }
                if (tanks.hasOwnProperty(deadId)) {
                    tanks[deadId].destroy();
                    if (deadId == myClientId) {
                        inputState.shootPower = 0;
                        inputState.jumpPower = 0;
                        showLogin(tanks[deadId].lifeFrags);
                    }
                    delete tanks[deadId];
                } else if (bullets.hasOwnProperty(deadId)) {
                    var bullet = bullets[deadId];
                    render.stage.removeChild(bullet.shape);
                    delete bullets[deadId];
                }
            }
        };

        setInterval(function() { // Sending packets to server
              var messageBuffer = new ArrayBuffer(6);
              var dataView = new DataView(messageBuffer);
              dataView.setUint8(0, MSG_OUT_MOVING);
              dataView.setUint8(1, inputState.wasMoving);
              if (inputState.wasChangingAngle == 0 &&
                  inputState.changingAngle != 0) {
                  inputState.wasChangingAngle = scaleTimeToByte(inputState.changingAngle, inputState.startChangingAngle, 200);
              }
              dataView.setUint8(2, inputState.wasChangingAngle);
              if (conn.readyState === conn.OPEN) {
                  conn.send(messageBuffer);
              } else {
                  //console.log("Still waiting for the conn to be established");
              }
              inputState.startChangingAngle = performance.now();
              inputState.wasChangingAngle = 0;
              inputState.wasMoving = inputState.moving;
        }, 200);
    }

    function getPower(timeStart, speed) {
        var power = Math.floor((Date.now() - timeStart) * speed);
        if (power <= 0) {
            power = 1;
        }
        if (power > 10) {
            power = 10;
        }
        return power;
    }

    window.addEventListener("keydown", function(e) {
        var keyCode = 'which' in e ? e.which : e.keyCode;
        if (keyCode == KEYCODE_ENTER &&
            document.getElementById("overlay").style.display != "none") {
            onPlayButtonClicked();
            return;
        }
        if (!me()) {
            return;
        }
        if (keyCode == KEYCODE_LEFT || keyCode == KEYCODE_RIGHT) {
            var newDir = (keyCode == KEYCODE_LEFT) ? -1 : 1;
            if (INTERPOLATION_ENABLED) {
                var newDir = (keyCode == KEYCODE_LEFT) ? -1 : 1;
                if (newDir != me().direction) {
                    me().gun.rotation = 180 - me().gun.rotation;
                    me().direction = newDir;
                }
            }
            inputState.moving = newDir;
        }
        if (keyCode == KEYCODE_UP || keyCode == KEYCODE_DOWN) {
            var rotationDirection = (keyCode == KEYCODE_UP ? 1 : -1);
            if (rotationDirection != inputState.changingAngle) {
                inputState.startChangingAngle = performance.now();
                inputState.changingAngle = rotationDirection; 
            }
        }
        if (keyCode == KEYCODE_C) {
            if (inputState.shootPower == 0) {
                inputState.shootPower = Date.now();
                var updateShootProgress = function () {
                    if (inputState.shootPower == 0) {
                        return;
                    }
                    var power = getPower(inputState.shootPower, SHOOT_POWERUP_SPEED);
                    if (power > 1) {
                        me().shootProgress.visible = true;
                        me().shootBar.visible = true;
                        me().shootProgress.graphics.clear().setStrokeStyle(2)
                            .beginStroke("red")
                            .drawRect(-18, 16, (37 * power / 10) | 0, 2);
                    }
                    me().gun.filters = [new createjs.ColorFilter(1, 1, 1, 1, power/10*150, 0, 0, 0)];
                    me().gun.cache(0, 0, 100, 100);
                    setTimeout(updateShootProgress, 1/SHOOT_POWERUP_SPEED);
                }
                updateShootProgress();
            }
        }
        if (keyCode == KEYCODE_X) {
            if (inputState.jumpPower == 0) {
                inputState.jumpPower = Date.now();
                var updateJumpProgress = function () {
                    if (inputState.jumpPower == 0) {
                        return;
                    }
                    var power = getPower(inputState.jumpPower, JUMP_POWERUP_SPEED);
                    if (power > 1) {
                        me().jumpProgress.visible = true;
                        me().jumpBar.visible = true;
                        me().jumpProgress.graphics.clear().setStrokeStyle(2)
                            .beginStroke("white")
                            .drawRect(-18, 23, (37 * power / 10) | 0, 2);
                    }
                    setTimeout(updateJumpProgress, 1/JUMP_POWERUP_SPEED);
                }
                updateJumpProgress();
            }
        }
    });

    function scaleTimeToByte(sign, startTime, delay) {
        var delta = performance.now() - startTime;
        if (delta > delay) {
            delta = delay;
        }
        return sign * (delta / delay) * 127;
    }

    window.addEventListener("keyup", function(e) {
        var keyCode = 'which' in e ? e.which : e.keyCode;
        if (!me()) {
            return;
        }
        if (keyCode == KEYCODE_LEFT || keyCode == KEYCODE_RIGHT) {
            var keyUpDir = (keyCode == KEYCODE_LEFT) ? -1 : 1;
            if (keyUpDir == inputState.moving) {
                inputState.wasMoving = inputState.moving;
                inputState.moving = 0;
            }
        }
        if (keyCode == KEYCODE_UP || keyCode == KEYCODE_DOWN) {
            inputState.wasChangingAngle = scaleTimeToByte(inputState.changingAngle, inputState.startChangingAngle, 200);
            inputState.startChangingAngle = 0;
            inputState.changingAngle = 0;
        }
        if (keyCode == KEYCODE_C) {
            inputState.shootPower = getPower(inputState.shootPower, SHOOT_POWERUP_SPEED);
            var messageBuffer = new ArrayBuffer(2);
            var dataView = new DataView(messageBuffer);
            dataView.setUint8(0, MSG_OUT_SHOOTING);
            dataView.setUint8(1, inputState.shootPower);
            conn.send(messageBuffer);
            me().shootProgress.visible = false;
            me().shootBar.visible = false;
            inputState.shootPower = 0;
            me().gun.filters = [];
            me().gun.cache(0, 0, 100, 100);
        }
        if (keyCode == KEYCODE_Z) {
            var messageBuffer = new ArrayBuffer(1);
            var dataView = new DataView(messageBuffer);
            dataView.setUint8(0, MSG_OUT_SHIELD);
            conn.send(messageBuffer);
        }
        if (keyCode == KEYCODE_X) {
            inputState.jumpPower = getPower(inputState.jumpPower, JUMP_POWERUP_SPEED);
            var messageBuffer = new ArrayBuffer(2);
            var dataView = new DataView(messageBuffer);
            dataView.setUint8(0, MSG_OUT_JUMP);
            dataView.setUint8(1, inputState.jumpPower);
            me().jumpProgress.visible = false;
            me().jumpBar.visible = false;
            inputState.jumpPower = 0;
            conn.send(messageBuffer);
        }
    });

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
    createjs.Ticker.setFPS(30);
    var deltaTime = 0;
    function tick(evt) {
        var curTime = performance.now();
        for (var id in bullets) {
            var bullet = bullets[id];
            bullet.updateState(curTime);
        }
        //changeAngle(inputState.changingAngle, 0.04 * deltaTime);
        render.redraw();
        var endTime = performance.now();
        deltaTime = endTime - startTime;
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

    var wsProtocol = (window.location.protocol === 'https:') ? 'wss:' : 'ws:';
    var overlordPath = (typeof window.overlordPath !== 'undefined') ? window.overlordPath : '/ws';
    var connectToOverlord = function (overlordHost, clb, retriesLeft) {
        if (retriesLeft === undefined) retriesLeft = 2;
        var fallbackGameUrl = wsProtocol + "//" + window.location.host + "/ws";
        var overlordBase = overlordHost ? (wsProtocol + "//" + overlordHost) : (wsProtocol + "//" + window.location.host);
        var overlordWSUrl = overlordBase + overlordPath;
        var overlordConn = new WebSocket(overlordWSUrl);
        overlordConn.binaryType = "arraybuffer";
        overlordConn.onclose = function (evt) {
            console.log("Overlord connection closed.")
        };
        overlordConn.onerror = function (evt) {
            console.log("Connection to overlord failed.");
            if (retriesLeft > 0) {
                console.log("Retrying overlord...");
                connectToOverlord(overlordHost, clb, retriesLeft - 1);
            } else {
                console.log("Using same host as game server: " + fallbackGameUrl);
                clb(fallbackGameUrl);
            }
        };
        overlordConn.onmessage = function (evt) {
            if (evt.data instanceof ArrayBuffer) {
                // TODO: ping handler
            } else {
                console.log("Server list received from overlord");
                var lines = evt.data.split('\n');
                var bestServer = null, bestPlayers = -1;
                for (var i = 0; i < lines.length; ++i) {
                    var fields = lines[i].split('\t');
                    var players = Number(fields[1]);
                    if (players > bestPlayers) {
                        bestServer = fields[0];
                    }
                }
                if (bestServer == null) {
                    console.log("No best server, retrying");
                    setTimeout(function () {
                        // reask for server list
                        var messageBuffer = new ArrayBuffer(1);
                        overlordConn.send(messageBuffer)
                    }, 500);
                } else {
                    console.log("Best server determined: " + bestServer);
                    overlordConn.close();
                    clb(bestServer);
                }
            }
        };
        overlordConn.onopen = function () {
            console.log("Overlord connection open");
            var messageBuffer = new ArrayBuffer(1);
            overlordConn.send(messageBuffer)
        };
    }

    var startConnectionPipeline = function () {
        connectToOverlord(overlord, function (bestServer) {
            connectToGameServer(bestServer);
        });
    };
    startConnectionPipeline();
};

function onPlayButtonClicked () {
    window.playButtonClicked++;
    var login = document.getElementById("loginInput").value;
    document.getElementById("overlay").style.display = "none";
    document.getElementById("login").style.display = "none";
    document.activeElement.blur();
    ga('send', 'event', 'game', 'playButton', 'clicked', playButtonClicked);
    sendStart(login);
}

