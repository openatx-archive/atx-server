# ATX-SERVER
Manage batch of atx-agents

# Usage
```
go run main.go -addr :8000
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
成功状态码200,失败403

```bash
$ curl -X POST $SERVER_URL/devices/{query}/reserved
Success
```

## 设备释放
成功状态码200,失败403

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