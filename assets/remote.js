/* Javascript */
window.app = new Vue({
  el: '#app',
  data: {
    deviceId: '',
    device: {
      ip: deviceIp,
      port: 7912,
    },
    fixConsole: '', // log for fix minicap and rotation
    navtabs: {
      active: location.hash.slice(1) || 'home',
      tabs: [],
    },
    error: '',
    control: null,
    loading: false,
    canvas: {
      bg: null,
      fg: null,
    },
    canvasStyle: {
      opacity: 1,
      width: 'inherit',
      height: 'inherit'
    },
    lastScreenSize: {
      screen: {},
      canvas: {
        width: 1,
        height: 1
      }
    },
    browserURL: "",
    logcat: {
      follow: true,
      tagColors: {},
      lineNumber: 0,
      maxKeep: 1500,
      cachedScrollTop: 0,
      logs: [{
        lineno: 1,
        tag: "EsService2",
        level: "W",
        content: "loaded /system/lib/egl/libEGL_adreno200.so",
      }]
    },
    imageBlobBuffer: [],
    videoUrl: '',
  },
  watch: {},
  computed: {
    deviceUrl: function () {
      return "http://" + this.device.ip + ":" + this.device.port;
    }
  },
  mounted: function () {
    var URL = window.URL || window.webkitURL;
    var currentSize = null;
    var self = this;
    $.notify.defaults({ className: "success" });

    this.canvas.bg = document.getElementById('bgCanvas')
    this.canvas.fg = document.getElementById('fgCanvas')
    // this.canvas = c;
    window.c = this.canvas.bg;
    var ctx = c.getContext('2d')

    $(window).resize(function () {
      self.resizeScreen();
    })

    this.initDragDealer();

    this.enableTouch();
    this.loadLiveScreen();

    // wakeup device on connect
    setTimeout(function () {
      this.keyevent("WAKEUP");
    }.bind(this), 1)

    window.k = setTimeout(function () {
      var lineno = (this.logcat.lineNumber += 1);
      this.logcat.logs.push({
        lineno: lineno,
        tag: "EsService2",
        level: "W",
        content: "loaded /system/lib/egl/libEGL_adreno200.so",
      });
      if (this.logcat.follow) {
        // only keep maxKeep lines
        var maxKeep = Math.max(20, this.logcat.maxKeep);
        var size = this.logcat.logs.length;
        this.logcat.logs = this.logcat.logs.slice(size - maxKeep, size);

        // scroll to end
        var el = this.$refs.tab_content;
        var logcat = this.logcat;
        if (el.scrollTop < logcat.cachedScrollTop) {
          this.logcat.follow = false;
        } else {
          setTimeout(function () {
            logcat.cachedScrollTop = el.scrollTop = el.scrollHeight - el.clientHeight;
          }, 2);
        }
      }
    }.bind(this), 200)
  },
  methods: {
    saveShortVideo: function (event) {
      var fd = new FormData();
      this.imageBlobBuffer.forEach(function (blob) {
        fd.append('file', blob);
      });
      $(event.target).notify("视频后台合成中，请稍候 ...");
      console.log("upload")
      $.ajax({
        type: "post",
        url: "http://10.246.46.160:7000/img2video", // TODO: 临时地址，需要后期更换
        processData: false,
        contentType: false,
        data: fd,
        dateType: 'json',
      }).done(function (data) {
        console.log(data.url);
        this.videoUrl = data.url;
        $(event.target).notify("合成完毕");
      }.bind(this))
    },
    saveScreenshot: function () {
      $.ajax({
        url: this.deviceUrl + "/screenshot",
        cache: false,
        xhrFields: {
          responseType: 'blob'
        },
      }).then(function (blob) {
        saveAs(blob, "screenshot.jpg") // saveAs require FileSaver.js
      })
    },
    openBrowser: function (url) {
      if (!/^https?:\/\//.test(url)) {
        url = "http://" + url;
      }
      return this.shell("am start -a android.intent.action.VIEW -d " + url);
    },
    uploadFile: function (event) {
      var formData = new FormData(event.target);
      $(event.target).notify("Uploading ...");
      $.ajax({
        method: "post",
        url: this.deviceUrl + "/upload/sdcard/tmp/",
        data: formData,
        processData: false,
        contentType: false,
        enctype: 'multipart/form-data',
      })
        .then(function (ret) {
          $(event.target).notify("Upload success");
        }, function (err) {
          $(event.target).notify("Upload failed:" + err.responseText, "error")
          console.error(err)
        })
    },
    addTabItem: function (item) {
      this.navtabs.tabs.push(item);
    },
    changeTab: function (tabId) {
      location.hash = tabId;
    },
    fixRotation: function () {
      $.ajax({
        url: this.deviceUrl + "/info/rotation",
        method: "post",
      }).then(function (ret) {
        console.log("rotation fixed")
      })
    },
    fixMinicap: function () {
      this.fixConsole = "remove old minicap";
      $.ajax({
        method: "post",
        url: this.deviceUrl + "/shell",
        data: {
          command: "rm -f /data/local/tmp/minicap /data/local/tmp/minicap.so"
        }
      })
        .then(function () {
          this.fixConsole = "download mincap to device ..."
          return $.ajax({
            url: this.deviceUrl + "/minicap",
            method: "put",
          })
        }.bind(this))
        .then(function () {
          this.fixConsole = "minicap fixed"
        }.bind(this), function () {
          this.fixConsole = "minicap can not be fixed, open Browser Console for more detail"
        }.bind(this))
    },
    tabScroll: function (ev) {
      // var el = ev.target;
      // var el = this.$refs.tab_content;
      // var bottom = (el.scrollTop == (el.scrollHeight - el.clientHeight));
      // console.log("Bottom", bottom, el.scrollTop, el.scrollHeight, el.clientHeight, el.scrollHeight - el.clientHeight)
      // console.log(ev.target.scrollTop, ev.target.scrollHeight, ev.target.clientHeight);
      this.logcat.follow = false;
    },
    followLog: function () {
      this.logcat.follow = !this.logcat.follow;
      if (this.logcat.follow) {
        var el = this.$refs.tab_content;
        el.scrollTop = el.scrollHeight - el.clientHeight;
      }
    },
    logcatTag2Color: function (tag) {
      var color = this.logcat.tagColors[tag];
      if (!color) {
        color = this.logcat.tagColors[tag] = getRandomRgb(5);
      }
      return color;
    },
    logcatLevel2Color: function (level) {
      switch (level) {
        case "W":
          return "goldenrod";
        case "I":
          return "darkgreen";
        case "D":
          return "gray";
        default:
          return "gray";
      }
    },
    hold: function (msecs) {
      this.control.touchDown(0, 0.5, 0.5, 5, 0.5)
      this.control.touchCommit();
      this.control.touchWait(msecs);
      this.control.touchUp(0)
      this.control.touchCommit();
    },
    keyevent: function (meta) {
      console.log("keyevent", meta)
      return this.shell("input keyevent " + meta.toUpperCase());
    },
    shell: function (command) {
      return $.ajax({
        url: this.deviceUrl + "/shell",
        method: "post",
        data: {
          command: command,
        },
        success: function (ret) {
          console.log(ret);
        },
        error: function (ret) {
          console.log(ret)
        }
      })
    },
    showError: function (error) {
      this.loading = false;
      this.error = error;
      $('.modal').modal('show');
    },
    showAjaxError: function (ret) {
      if (ret.responseJSON && ret.responseJSON.description) {
        this.showError(ret.responseJSON.description);
      } else {
        this.showError("<p>Local server not started, start with</p><pre>$ python -m weditor</pre>");
      }
    },
    initDragDealer: function () {
      var self = this;
      var updateFunc = null;

      function dragMoveListener(evt) {
        evt.preventDefault();
        updateFunc(evt);
        self.resizeScreen();
      }

      function dragStopListener(evt) {
        document.removeEventListener('mousemove', dragMoveListener);
        document.removeEventListener('mouseup', dragStopListener);
        document.removeEventListener('mouseleave', dragStopListener);
      }

      $('#vertical-gap1').mousedown(function (e) {
        e.preventDefault();
        updateFunc = function (evt) {
          $("#left").width(evt.clientX);
        }
        document.addEventListener('mousemove', dragMoveListener);
        document.addEventListener('mouseup', dragStopListener);
        document.addEventListener('mouseleave', dragStopListener)
      });
    },
    resizeScreen: function (img) {
      // check if need update
      if (img) {
        if (this.lastScreenSize.canvas.width == img.width &&
          this.lastScreenSize.canvas.height == img.height) {
          return;
        }
      } else {
        img = this.lastScreenSize.canvas;
        if (!img) {
          return;
        }
      }
      var screenDiv = document.getElementById('screen');
      this.lastScreenSize = {
        canvas: {
          width: img.width,
          height: img.height
        },
        screen: {
          width: screenDiv.clientWidth,
          height: screenDiv.clientHeight,
        }
      }

      var canvasAspect = img.width / img.height;
      var screenAspect = screenDiv.clientWidth / screenDiv.clientHeight;
      if (canvasAspect > screenAspect) {
        Object.assign(this.canvasStyle, {
          width: Math.floor(screenDiv.clientWidth) + 'px', //'100%',
          height: Math.floor(screenDiv.clientWidth / canvasAspect) + 'px', // 'inherit',
        })
      } else if (canvasAspect < screenAspect) {
        Object.assign(this.canvasStyle, {
          width: Math.floor(screenDiv.clientHeight * canvasAspect) + 'px', //'inherit',
          height: Math.floor(screenDiv.clientHeight) + 'px', //'100%',
        })
      }
    },
    delayReload: function (msec) {
      setTimeout(this.screenDumpUI, msec || 1000);
    },
    drawBlobImageToScreen: function (blob) {
      // Support jQuery Promise
      var dtd = $.Deferred();
      var bgcanvas = this.canvas.bg,
        fgcanvas = this.canvas.fg,
        ctx = bgcanvas.getContext('2d'),
        self = this,
        URL = window.URL || window.webkitURL,
        BLANK_IMG = 'data:image/gif;base64,R0lGODlhAQABAAAAACH5BAEKAAEALAAAAAABAAEAAAICTAEAOw==',
        img = this.imagePool.next();

      img.onload = function () {
        console.log("image")
        fgcanvas.width = bgcanvas.width = img.width
        fgcanvas.height = bgcanvas.height = img.height


        ctx.drawImage(img, 0, 0, img.width, img.height);
        self.resizeScreen(img);

        // Try to forcefully clean everything to get rid of memory
        // leaks. Note self despite this effort, Chrome will still
        // leak huge amounts of memory when the developer tools are
        // open, probably to save the resources for inspection. When
        // the developer tools are closed no memory is leaked.
        img.onload = img.onerror = null
        img.src = BLANK_IMG
        img = null
        blob = null

        URL.revokeObjectURL(url)
        url = null
        dtd.resolve();
      }

      img.onerror = function () {
        // Happily ignore. I suppose this shouldn't happen, but
        // sometimes it does, presumably when we're loading images
        // too quickly.

        // Do the same cleanup here as in onload.
        img.onload = img.onerror = null
        img.src = BLANK_IMG
        img = null
        blob = null

        URL.revokeObjectURL(url)
        url = null
        dtd.reject();
      }
      var url = URL.createObjectURL(blob)
      img.src = url;
      return dtd;
    },
    loadLiveScreen: function () {
      var self = this;
      var BLANK_IMG =
        'data:image/gif;base64,R0lGODlhAQABAAAAACH5BAEKAAEALAAAAAABAAEAAAICTAEAOw=='
      var protocol = location.protocol == "http:" ? "ws://" : "wss://"
      var ws = new WebSocket(this.deviceUrl.replace("http:", "ws:") + '/minicap');
      var canvas = document.getElementById('bgCanvas')
      var ctx = canvas.getContext('2d');
      var lastScreenSize = {
        screen: {},
        canvas: {}
      };

      this.screenWS = ws;
      var imagePool = new ImagePool(100);

      ws.onopen = function (ev) {
        console.log('screen websocket connected')
      };

      var imageBlobBuffer = self.imageBlobBuffer;
      var imageBlobMaxLength = 300;

      ws.onmessage = function (message) {
        if (message.data instanceof Blob) {
          console.log("image received");

          var blob = new Blob([message.data], {
            type: 'image/jpeg'
          })

          imageBlobBuffer.push(blob);
          if (imageBlobBuffer.length > imageBlobMaxLength) {
            imageBlobBuffer.shift();
          }

          var img = imagePool.next();
          img.onload = function () {
            canvas.width = img.width
            canvas.height = img.height
            ctx.drawImage(img, 0, 0, img.width, img.height);
            self.resizeScreen(img);


            // Try to forcefully clean everything to get rid of memory
            // leaks. Note self despite this effort, Chrome will still
            // leak huge amounts of memory when the developer tools are
            // open, probably to save the resources for inspection. When
            // the developer tools are closed no memory is leaked.
            img.onload = img.onerror = null
            img.src = BLANK_IMG
            img = null
            blob = null

            URL.revokeObjectURL(url)
            url = null
          }

          img.onerror = function () {
            // Happily ignore. I suppose this shouldn't happen, but
            // sometimes it does, presumably when we're loading images
            // too quickly.

            // Do the same cleanup here as in onload.
            img.onload = img.onerror = null
            img.src = BLANK_IMG
            img = null
            blob = null

            URL.revokeObjectURL(url)
            url = null
          }

          var url = URL.createObjectURL(blob)
          img.src = url;
        } else if (/^data size:/.test(message.data)) {
          // console.log("receive message:", message.data)
        } else if (/^rotation/.test(message.data)) {
          self.rotation = parseInt(message.data.substr('rotation '.length), 10);
          console.log(self.rotation)
        } else {
          console.log("receive message:", message.data)
        }
      }

      ws.onclose = function (ev) {
        console.log("screen websocket closed", ev.code)
      }

      ws.onerror = function (ev) {
        console.log("screen websocket error")
      }
    },
    enableTouch: function () {
      /**
       * TOUCH HANDLING
       */
      var self = this;
      var element = this.canvas.fg;

      var screen = {
        bounds: {}
      }

      var ws = new WebSocket(this.deviceUrl.replace("http:", "ws:") + "/minitouch")
      ws.onerror = function (ev) {
        console.log("minitouch websocket error:", ev)
      }
      ws.onmessage = function (ev) {
        console.log("minitouch websocket receive message:", ev.data);
      }
      ws.onclose = function () {
        console.log("minitouch websocket closed");
      }
      var control = this.control = MiniTouch.createNew(ws);

      function calculateBounds() {
        var el = element;
        screen.bounds.w = el.offsetWidth
        screen.bounds.h = el.offsetHeight
        screen.bounds.x = 0
        screen.bounds.y = 0

        while (el.offsetParent) {
          screen.bounds.x += el.offsetLeft
          screen.bounds.y += el.offsetTop
          el = el.offsetParent
        }
      }

      function activeFinger(index, x, y, pressure) {
        var scale = 0.5 + pressure
        $(".finger-" + index)
          .addClass("active")
          .css("transform", 'translate3d(' + x + 'px,' + y + 'px,0)')
      }

      function deactiveFinger(index) {
        $(".finger-" + index).removeClass("active")
      }

      function mouseDownListener(event) {
        var e = event;
        if (e.originalEvent) {
          e = e.originalEvent
        }
        // Skip secondary click
        if (e.which === 3) {
          return
        }
        e.preventDefault()

        fakePinch = e.altKey
        calculateBounds()

        var x = e.pageX - screen.bounds.x
        var y = e.pageY - screen.bounds.y
        var pressure = 0.5
        activeFinger(0, e.pageX, e.pageY, pressure);

        var scaled = coords(screen.bounds.w, screen.bounds.h, x, y, self.rotation);
        control.touchDown(0, scaled.xP, scaled.yP, pressure);
        control.touchCommit();

        element.removeEventListener('mousemove', mouseHoverListener);
        element.addEventListener('mousemove', mouseMoveListener);
        document.addEventListener('mouseup', mouseUpListener);
      }

      function mouseMoveListener(event) {
        var e = event
        if (e.originalEvent) {
          e = e.originalEvent
        }
        // Skip secondary click
        if (e.which === 3) {
          return
        }
        e.preventDefault()

        var pressure = 0.5
        activeFinger(0, e.pageX, e.pageY, pressure);
        var x = e.pageX - screen.bounds.x
        var y = e.pageY - screen.bounds.y
        var scaled = coords(screen.bounds.w, screen.bounds.h, x, y, self.rotation);
        control.touchMove(0, scaled.xP, scaled.yP, pressure);
        control.touchCommit();
      }

      function mouseUpListener(event) {
        var e = event
        if (e.originalEvent) {
          e = e.originalEvent
        }
        // Skip secondary click
        if (e.which === 3) {
          return
        }
        e.preventDefault()

        control.touchUp(0)
        control.touchCommit();
        stopMousing()
      }

      function stopMousing() {
        element.removeEventListener('mousemove', mouseMoveListener);
        // element.addEventListener('mousemove', mouseHoverListener);
        document.removeEventListener('mouseup', mouseUpListener);
        deactiveFinger(0);
      }

      // function coord(event) {
      //   var e = event;
      //   if (e.originalEvent) {
      //     e = e.originalEvent
      //   }
      //   calculateBounds()
      //   var x = e.pageX - screen.bounds.x
      //   var y = e.pageY - screen.bounds.y
      //   var px = x / screen.bounds.w;
      //   var py = y / screen.bounds.h;
      //   return {
      //     px: px,
      //     py: py,
      //     x: Math.floor(px * element.width),
      //     y: Math.floor(py * element.height),
      //   }
      // }

      function mouseHoverListener(event) {
        var e = event;
        if (e.originalEvent) {
          e = e.originalEvent
        }
        // Skip secondary click
        if (e.which === 3) {
          return
        }
        e.preventDefault()

        var x = e.pageX - screen.bounds.x
        var y = e.pageY - screen.bounds.y
      }

      function markPosition(pos) {
        var ctx = self.canvas.fg.getContext("2d");
        ctx.fillStyle = '#ff0000'; // red
        ctx.beginPath()
        ctx.arc(pos.x, pos.y, 12, 0, 2 * Math.PI)
        ctx.closePath()
        ctx.fill()

        ctx.fillStyle = "#fff"; // white
        ctx.beginPath()
        ctx.arc(pos.x, pos.y, 8, 0, 2 * Math.PI)
        ctx.closePath()
        ctx.fill();
      }

      var wheelTimer, fromYP;

      function mouseWheelDelayTouchUp() {
        clearTimeout(wheelTimer);
        wheelTimer = setTimeout(function () {
          fromYP = null;
          control.touchUp(1)
          control.touchCommit();
          // deactiveFinger(0);
          // deactiveFinger(1);
        }, 100)
      }

      function mouseWheelListener(event) {
        var e = event;
        if (e.originalEvent) {
          e = e.originalEvent
        }
        e.preventDefault()
        calculateBounds()

        var x = e.pageX - screen.bounds.x
        var y = e.pageY - screen.bounds.y
        var pressure = 0.5;
        var scaled;

        if (!fromYP) {
          fromYP = y / screen.bounds.h; // display Y percent
          // touch down when first detect mousewheel
          scaled = coords(screen.bounds.w, screen.bounds.h, x, y, self.rotation);
          control.touchDown(1, scaled.xP, scaled.yP, pressure);
          control.touchCommit();
          // activeFinger(0, e.pageX, e.pageY, pressure);
        }
        // caculate position after scroll
        var toYP = fromYP + (event.wheelDeltaY < 0 ? -0.05 : 0.05);
        toYP = Math.max(0, Math.min(1, toYP));

        var step = Math.max((toYP - fromYP) / 5, 0.01) * (event.wheelDeltaY < 0 ? -1 : 1);
        for (var yP = fromYP; yP < 1 && yP > 0 && Math.abs(yP - toYP) > 0.0001; yP += step) {
          y = screen.bounds.h * yP;
          var pageY = y + screen.bounds.y;
          scaled = coords(screen.bounds.w, screen.bounds.h, x, y, self.rotation);
          // activeFinger(1, e.pageX, pageY, pressure);
          control.touchMove(1, scaled.xP, scaled.yP, pressure);
          control.touchWait(10);
          control.touchCommit();
        }
        fromYP = toYP;
        mouseWheelDelayTouchUp()
      }

      /* bind listeners */
      element.addEventListener('mousedown', mouseDownListener);
      element.addEventListener('mousemove', mouseHoverListener);
      element.addEventListener('mousewheel', mouseWheelListener);
    }
  }
})