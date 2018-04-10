# coding: utf-8
#
# Py3 only
from __future__ import print_function

import os
import uuid
import pathlib
import shutil
import time
import io
import json
import traceback

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
            for p in sorted(
                    videopath.glob('*.mp4'), key=lambda p: p.stat().st_mtime):
                info = {
                    'uri': str(p).replace('\\', '/'),
                    'mtime': p.stat().st_mtime,
                }
                meta = pathlib.Path(str(p) + ".json")
                if meta.exists():
                    with meta.open('rb') as f:  # read_text not exists on py3.4
                        metainfo = json.loads(f.read().decode('utf-8'))
                        info.update(metainfo)
                data.append(info)
            self.write({'data': list(reversed(data))})
            return
        self.render("videos.html")

    def delete(self, name):
        mp4file = pathlib.Path("static/videos/" + name)
        mp4meta = pathlib.Path("static/videos/" + name + ".json")
        if mp4meta.exists():
            mp4meta.unlink()
        if mp4file.exists():
            mp4file.unlink()
            self.write({"success": True})
        else:
            self.write({"success": False})


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
    return im.resize(size, Image.ANTIALIAS)


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
        self.udid = self.get_argument(
            'udid')  # device udid(unique device identifier)
        self.name = self.get_argument('name')
        self.video_path = 'static/videos/%s-%d.mp4' % (self.name,
                                                       int(time.time() * 1000))
        self.video_tmp_path = 'tmpdir/ws-%s.mp4' % str(uuid.uuid1())
        fps = int(self.get_argument('fps', 10))
        self.writer = imageio.get_writer(self.video_tmp_path, fps=fps)
        self.size = ()
        print("websocket opened")

    def on_message(self, message):
        if isinstance(message, bytes):
            try:
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
            except Exception as e:
                print("Receive image format error")
                traceback.print_exc()
        else:
            print("receive", message)

    def on_close(self):
        self.writer.close()
        if not os.path.exists(self.video_tmp_path):
            print("no video file generated")
            return
        shutil.move(self.video_tmp_path, self.video_path)
        with open(self.video_path + '.json', 'wb') as f:
            f.write(
                json.dumps({
                    "udid": self.udid,
                    "name": self.name
                }).encode('utf-8'))
        print("websocket closed, video generated", self.video_path)


class Image2VideoHandler(CorsMixin, tornado.web.RequestHandler):
    @gen.coroutine
    def post(self):
        filemetas = self.request.files['file']
        tmpdir = pathlib.Path('tmpdir/' + str(uuid.uuid1()))
        if not tmpdir.is_dir():
            tmpdir.mkdir(parents=True)

        video_file = 'static/video-%s.mp4' % int(time.time() * 1000)
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
        (r"/videos/([^/]+)", VideoHandler),
        (r"/video/convert", Image2VideoWebsocket),
        (r"/img2video", Image2VideoHandler),
    ], **settings)


if __name__ == "__main__":
    hotreload = bool(os.getenv("DEBUG"))
    app = make_app(debug=hotreload)
    app.listen(7000)
    try:
        tornado.ioloop.IOLoop.instance().start()
    except KeyboardInterrupt:
        tornado.ioloop.IOLoop.instance().stop()