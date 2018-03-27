# coding: utf-8
from __future__ import print_function

import os
import uuid
import pathlib
import shutil
import time
import io

import numpy as np
import imageio
import tornado.ioloop
import tornado.web
import tornado.websocket
from PIL import Image, ImageOps
from tornado import gen
from tornado.httpclient import AsyncHTTPClient
from tornado.log import enable_pretty_logging
enable_pretty_logging()


class MainHandler(tornado.web.RequestHandler):
    @gen.coroutine
    def get(self):
        yield gen.sleep(.1)
        self.render("index.html")


class VideoHandler(tornado.web.RequestHandler):
    def get(self):
        content_type = self.request.headers.get('Content-Type')
        if content_type and 'application/json' in content_type:
            videopath = pathlib.Path("static/videos")
            data = []
            for p in videopath.glob('*.mp4'):
                data.append({
                    'name': str(p).replace('\\', '/'),
                    'mtime': p.stat().st_mtime,
                })
            self.write({'data': list(reversed(data))})
            return
        self.render("videos.html")


def resizefit(im, size):  # resize but keep aspect ratio
    w, h = size
    oldw, oldh = old_size = im.size
    old_ratio = oldw / oldh
    new_ratio = w / h
    if new_ratio > old_ratio:
        padw = int(oldh * new_ratio - oldw) // 2
        im = ImageOps.expand(im, (padw, 0, padw, 0), (0, 0, 255))
    else:
        padh = int(oldw / new_ratio - oldh) // 2
        im = ImageOps.expand(im, (0, padh, 0, padh), (0, 255, 255))
    return im.resize(size)


class CorsMixin(object):
    def set_default_headers(self):
        self.set_header("Access-Control-Allow-Origin", "*")
        self.set_header("Access-Control-Allow-Headers", "x-requested-with")
        self.set_header('Access-Control-Allow-Methods', 'POST, GET, OPTIONS')

    def options(self):
        # no body
        self.set_status(204)
        self.finish()


class Image2VideoWebsocket(tornado.websocket.WebSocketHandler):
    def check_origin(self, origin):
        return True

    def open(self):
        self.video_path = 'static/videos/ws-tmp-%d.mp4' % int(
            time.time() * 1000)
        self.writer = imageio.get_writer(self.video_path, fps=10)
        self.size = ()
        print("websocket opened")

    def on_message(self, message):
        if isinstance(message, bytes):
            # print("receive image")
            image = Image.open(io.BytesIO(message))
            if not self.size:  # always horizontal
                w, h = self.size = image.size
                if w < h:
                    self.size = (h, w)
            if self.size != image.size:
                image = resizefit(image, self.size)
            imarray = np.asarray(image)
            del image
            self.writer.append_data(imarray)
        else:
            print("receive", message)

    def on_close(self):
        self.writer.close()
        print("websocket closed, video genreated", self.video_path)


class Image2VideoHandler(CorsMixin, tornado.web.RequestHandler):
    @gen.coroutine
    def post(self):
        filemetas = self.request.files['file']
        tmpdir = pathlib.Path('tmpdir/' + str(uuid.uuid1()))
        if not tmpdir.is_dir():
            tmpdir.mkdir(parents=True)

        video_file = 'static/video-%s.mp4' % int(time.time() * 1000)
        # video_file = 'video.mp4'
        writer = imageio.get_writer(video_file, fps=20)
        try:
            size = ()
            for (i, meta) in enumerate(filemetas):
                jpgfile = tmpdir / ('%d.jpg' % i)
                with jpgfile.open('wb') as f:
                    f.write(meta['body'])
                imarray = imageio.imread(str(jpgfile))
                if not size:
                    size = imarray.shape[1::-1]  # same as reversed(shape[:2])
                if size != imarray.shape[1::-1]:
                    im = Image.fromarray(imarray)  # convert to PIL
                    im = resizefit(im, size)
                    imarray = np.asarray(im)  # convert to numpy
                    del im
                writer.append_data(imarray)
        finally:
            shutil.rmtree(str(tmpdir))
            writer.close()

        self.write({
            "success": True,
            "url": "http://" + self.request.host + "/" + video_file
        })


def make_app(**settings):
    settings['template_path'] = 'templates'
    settings['static_path'] = 'static'
    settings['cookie_secret'] = os.environ.get("SECRET", "SECRET:_")
    settings['login_url'] = '/login'
    return tornado.web.Application([
        (r"/", MainHandler),
        (r"/videos", VideoHandler),
        (r"/img2video", Image2VideoHandler),
        (r"/websocket", Image2VideoWebsocket),
    ], **settings)


if __name__ == "__main__":
    hotreload = bool(os.getenv("DEBUG"))
    app = make_app(debug=hotreload)
    app.listen(7000)
    try:
        tornado.ioloop.IOLoop.instance().start()
    except KeyboardInterrupt:
        tornado.ioloop.IOLoop.instance().stop()