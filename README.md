# ATX-SERVER
[![GitHub stars](https://img.shields.io/badge/govendor-vendor-blue.svg)](https://github.com/kardianos/govendor)
[![Build Status](https://travis-ci.org/openatx/atx-server.svg?branch=master)](https://travis-ci.org/openatx/atx-server)

Manage batch of atx-agents

# Testerhome上相关文章
- [安卓设备集群管理 atx-server](https://testerhome.com/topics/11546) By [codeskyblue](https://testerhome.com/codeskyblue)
- [atx 安卓集群管理 安装运行及自动化的实践](https://testerhome.com/topics/11588) By [cynic]: (https://testerhome.com/cynic)

# Install
重要：需要有go语言的基础，知道该如何编译一个go的程序

1. Install and start [rethinkdb](https://rethinkdb.com)
2. Install [go](https://golang.org)

Compile with go

```bash
$ go get -v github.com/openatx/atx-server
$ cd $GOPATH/src/github.com/openatx/atx-server
$ go build
```

# Usage
launch `rethinkdb`

```bash
$ rethinkdb
Running rethinkdb 2.3.6 (CLANG 8.1.0 (clang-802.0.42))...
Running on Darwin 16.6.0 x86_64
...
```

launch `atx-server`

```
./atx-server --port 8000
```

Install `atx-agent` using [uiautomator2](https://github.com/openatx/uiautomator2) into android phone. your android phone and server running `atx-server` should in the same intranet.

Suppose server running `atx-server` got the ip `10.0.1.1`, listen port `8000`. Do the following command

```bash
$ pip install -U --pre uiautomator2
$ python -m uiautomator2 init 10.0.1.1:8000
```

open browser <http://localhost:8000>, you should see the device listed on the web.

## Advanced usage
### Set up <https://www.dingtalk.com> notification.
1. Usage command flag

    ```
    ./atx-server --ding-token 13gb4db7c276d22e84f788fa693b729d53218b8e07d6ede43de79360c962 --port 8080
    ```

2. Set up env var

    ```
    export DING_TOKEN="13gb4db7c276d22e84f788fa693b729d53218b8e07d6ede43de79360c962"
    ./atx-server --port 8080
    ```

# APIs
## /list 接口

其中udid是通过hwaddr, model, serial组合生成的

```bash
$ curl $SERVER_URL/list
[
    {
        "udid": "741AEDR42P6YM-2c:57:31:4b:40:74-M2_E",
        "ip": "10.240.218.20",
        "present": true,
        "ready": true,
        "using": true,
        "provider": null,
        "serial": "741AEDR42P6YM",
        "brand": "Meizu",
        "model": "M2 E",
        "hwaddr": "2c:57:31:4b:40:74",
        "agentVersion": "0.1.1",
        "battery": {},
        "display": {
            "width": 1080,
            "height": 1920
        }
    }
]
```

There are some fields you need pay attention.

- `present` means device is online
- `ready` is the thumb :thumbsup: you can see and edit in the web
- `using` means if device is using by someone

`provider` is a special field, if device is plugged into some machine which running [u2init](https://github.com/openatx/u2init), the bellow info can be found in device info.

```json
"provider": {
    "id": "33576428",
    "ip": "10.0.0.1",
    "port": 10000,
    "present": true  # provider online of not
}
```

if `provider` is `null` it means device is not plugged-in.

## /devices/{query}/info
```bash
$ curl $SERVER_URL/devices/ip:10.0.0.1/info
# or
$ curl $SERVER_URL/devices/$UDID/info
```

返回值同/list的的单个结果，这里就不写了。

## /version
`atx-agent`通过检测该接口确定是否升级

```bash
$ curl /version
{
    "server": "dev",
    "atx-agent": "0.0.7"
}
```

## 执行shell命令
```bash
$ curl -X POST -F command="pwd" $SERVER_URL/devices/{query}/shell
{
    "output": "/"
}
```

## 设备管理
占用、释放

状态码 成功200,失败403

### 占用设备
```bash
$ curl -X POST $SERVER_URL/devices/{query}/reserved
Success
```

### 释放设备
状态码 成功200,失败403

```bash
$ curl -X DELETE $SERVER_URL/devices/{query}/reserved
Release success
```

随机占用一台设备

```bash
$ curl -X POST $SERVER_URL/devices/:random/reserved
Success
```

## Communication between provider(u2init) and server(atx-server)
Provider send POST to Server
**heartbeat info** to let server known provider is online. It is also need to send the same data to Server in 15s or the Provider will be marked offline.

```bash
$ curl -X POST -F id=$PROVIDER_ID -F port=11000 $SERVER_URL/provider/heartbeat
```

You may need to add ip field if provider and server is not in the same network

```bash
$ PROVIDER_IP=10.0.0.1 # change to your provider ip
$ PROVIDER_ID=ccdd11ff # change to your provider id
$ curl -X POST \
    -F ip=$PROVIDER_IP \
    -F id=$PROVIDER_ID \
    -F port=11000 \
    $SERVER_URL/provider/heartbeat
```

Server response status 200 indicate success, or 400 and else means failure

Send using bellow command when there is device plugged-in

```bash
$ DEVICE_UDID="3578298f-b4:0b:44:e6:1f:90-OD103" # change to your device udid
$ DATA="{\"status\": \"online\", \"udid\": \"$DEVICE_UDID\"}"
$ curl -X POST \
    -F id=$PROVIDER_ID \
    -F port=11000 \
    -F data="$DATA"  $SERVER_URL/provider/heartbeat
```

## Comminication between atx-agent and atx-server
It is complicated. Hard to write.

# Docker
`atx-server` is dockerized (based on `golang` image) and depends on the official `rethinkdb` container. To build and run all services, use:
```bash
docker-compuse up --build
```
`atx-server` can be accessed from `localhost:8000` and `rethinkdb` web console is available at `localhost:8001`, both specified in the compose file.
`rethinkdb` data is stored at `$PWD/data` (host volume). 

## References and some good resources
- Golang library for rethinkdb [gorethink](https://github.com/GoRethink/gorethink)
- [美团点评云真机平台实践](https://tech.meituan.com/cloud_phone.html)
- [腾讯TMQ-远程移动测试平台对比分析](https://blog.csdn.net/TMQ1225/article/details/52369171)
- [藏经阁-iOS多机远程控制技术](http://www.sohu.com/a/240584209_744135)

# LICENSE
[MIT](LICENSE)
