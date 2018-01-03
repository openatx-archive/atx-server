/* Javascript */
var app = new Vue({
  el: '#app',
  data: {
    deviceId: '',
    device: {
      ip: deviceIp,
      port: 7912,
    },
    console: {
      content: '',
    },
    error: '',
    control: null,
    imagePool: null,
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
  },
  watch: {},
  computed: {},
  created: function() {
    this.imagePool = new ImagePool(100);
  },
  mounted: function() {
    var URL = window.URL || window.webkitURL;
    var currentSize = null;
    var self = this;

    this.canvas.bg = document.getElementById('bgCanvas')
    this.canvas.fg = document.getElementById('fgCanvas')
      // this.canvas = c;
    window.c = this.canvas.bg;
    var ctx = c.getContext('2d')

    $(window).resize(function() {
      self.resizeScreen();
    })

    this.initDragDealer();

    this.enableTouch();
    this.loadLiveScreen();
  },
  methods: {
    hold: function(msecs) {
      this.control.touchDown(0, 0.5, 0.5, 5, 0.5)
      this.control.touchCommit();
      this.control.touchWait(msecs);
      this.control.touchUp(0)
      this.control.touchCommit();
    },
    keyevent: function(meta) {
      console.log("keyevent", meta)
      $.ajax({
        url: "http://" + this.device.ip + ":" + this.device.port + "/shell",
        method: "post",
        data: {
          command: "input keyevent " + meta.toUpperCase(),
        },
        success: function(ret) {
          console.log(ret);
        },
        error: function(ret) {
          console.log(ret)
        }
      })
    },
    showError: function(error) {
      this.loading = false;
      this.error = error;
      $('.modal').modal('show');
    },
    showAjaxError: function(ret) {
      if (ret.responseJSON && ret.responseJSON.description) {
        this.showError(ret.responseJSON.description);
      } else {
        this.showError("<p>Local server not started, start with</p><pre>$ python -m weditor</pre>");
      }
    },
    initDragDealer: function() {
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

      $('#vertical-gap1').mousedown(function(e) {
        e.preventDefault();
        updateFunc = function(evt) {
          $("#left").width(evt.clientX);
        }
        document.addEventListener('mousemove', dragMoveListener);
        document.addEventListener('mouseup', dragStopListener);
        document.addEventListener('mouseleave', dragStopListener)
      });
    },
    resizeScreen: function(img) {
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
    delayReload: function(msec) {
      setTimeout(this.screenDumpUI, msec || 1000);
    },
    drawBlobImageToScreen: function(blob) {
      // Support jQuery Promise
      var dtd = $.Deferred();
      var bgcanvas = this.canvas.bg,
        fgcanvas = this.canvas.fg,
        ctx = bgcanvas.getContext('2d'),
        self = this,
        URL = window.URL || window.webkitURL,
        BLANK_IMG = 'data:image/gif;base64,R0lGODlhAQABAAAAACH5BAEKAAEALAAAAAABAAEAAAICTAEAOw==',
        img = this.imagePool.next();

      img.onload = function() {
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

      img.onerror = function() {
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
    loadLiveScreen: function() {
      var self = this;
      var BLANK_IMG =
        'data:image/gif;base64,R0lGODlhAQABAAAAACH5BAEKAAEALAAAAAABAAEAAAICTAEAOw=='
      var protocol = location.protocol == "http:" ? "ws://" : "wss://"
      var ws = new WebSocket('ws://' + this.device.ip + ':' + this.device.port + '/minicap');
      var canvas = document.getElementById('bgCanvas')
      var ctx = canvas.getContext('2d');
      var lastScreenSize = {
        screen: {},
        canvas: {}
      };

      this.screenWS = ws;

      ws.onopen = function(ev) {
        console.log('screen websocket connected')
      };

      ws.onmessage = function(message) {
        if (message.data instanceof Blob) {
          console.log("New image");

          var blob = new Blob([message.data], {
            type: 'image/jpeg'
          })
          var img = self.imagePool.next();
          img.onload = function() {
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

          img.onerror = function() {
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

      ws.onclose = function(ev) {
        console.log("screen websocket closed")
      }
    },
    enableTouch: function() {
      /**
       * TOUCH HANDLING
       */
      var self = this;
      var element = this.canvas.fg;

      var screen = {
        bounds: {}
      }

      var ws = new WebSocket("ws://" + this.device.ip + ':' + this.device.port + "/minitouch")
      ws.onerror = function(ev) {
        console.log("minitouch websocket error:", ev)
      }
      ws.onmessage = function(ev) {
        console.log("minitouch websocket receive message:", ev.data);
      }
      ws.onclose = function() {
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
          // startMousing()

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
        var rotation = 0;
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

        var pos = coord(e);
        // change precision
        pos.px = Math.floor(pos.px * 1000) / 1000;
        pos.py = Math.floor(pos.py * 1000) / 1000;
        pos.x = Math.floor(pos.px * element.width);
        pos.y = Math.floor(pos.py * element.height);

        control.touchUp(0)
        control.touchCommit();
        stopMousing()
      }

      function stopMousing() {
        element.removeEventListener('mousemove', mouseMoveListener);
        element.addEventListener('mousemove', mouseHoverListener);
        document.removeEventListener('mouseup', mouseUpListener);
        deactiveFinger(0);
      }

      function coord(event) {
        var e = event;
        if (e.originalEvent) {
          e = e.originalEvent
        }
        calculateBounds()
        var x = e.pageX - screen.bounds.x
        var y = e.pageY - screen.bounds.y
        var px = x / screen.bounds.w;
        var py = y / screen.bounds.h;
        return {
          px: px,
          py: py,
          x: Math.floor(px * element.width),
          y: Math.floor(py * element.height),
        }
      }

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
          // startMousing()

        var x = e.pageX - screen.bounds.x
        var y = e.pageY - screen.bounds.y
        var pos = coord(event);
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

      /* bind listeners */
      element.addEventListener('mousedown', mouseDownListener);
      element.addEventListener('mousemove', mouseHoverListener);
    }
  }
})