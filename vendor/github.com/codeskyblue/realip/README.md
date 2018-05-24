# RealIP
Get real IP behide nginx (Do not ignore private address)

Follows the rule of `X-Forwardd-For` and `X-Real-IP`

Refs: <https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For>

## Install
```
go get -v github.com/codeskyblue/realip
```

## Usage
```go
package main

import "github.com/codeskyblue/realip"

func (h *Handler) ServeIndexPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	clientIP := realip.FromRequest(r)
	log.Println("GET / from", clientIP)
}
``` 

## Thanks to
- <https://github.com/tomasen/realip>

# LICENSE
[MIT](LICENSE)