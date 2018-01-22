# ATX-SERVER
Manage batch of atx-agents

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
        "serial": "741AEDR42P6YM",
        "brand": "Meizu",
        "model": "M2 E",
        "hwaddr": "2c:57:31:4b:40:74",
        "ip": "10.240.218.20",
        "agentVersion": "0.1.1",
        "battery": {},
        "display": {
            "width": 1080,
            "height": 1920
        }
    }
]
```

## /devices/{query}/info
```bash
$ curl $SERVER_URL/devices/ip:10.0.0.1/info
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

## 设备占用
状态码 成功200,失败403

```bash
$ curl -X POST $SERVER_URL/devices/{query}/reserved
Success
```

## 设备释放
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

# LICENSE
[MIT](LICENSE)