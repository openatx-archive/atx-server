# dingrobot
DingTake Robot API

Prepare DingTalk robot access_token, See my article <https://testerhome.com/topics/11217> if you dont know how to get it.

## Usage
```go
import "github.com/codeskyblue/dingrobot"

func main(){
    robot := dingrobot.New("xxxxx-access_token-*****")
    // send text message
    robot.Text("Hi")
    // send markdown message
    robot.Markdown("**Hi**")
    // send link
    robot.Link("Google", "Google homepage", "https://www.google.com.hk")

    // At someone
    robot.At("13811223344").Text("test at someone")

    // At all
    robot.AtAll(true).Text("test at all message")
}
```

## TODO
* FeedCard
* ActionCard
* ImageUpload

## LICENSE
[MIT](LICENSE)