# Fat Bot
The Telegram bot that keeps you lean
---

![alt text](http://omer.hamerman.co.s3.amazonaws.com/putin.png)

### Getting Started
* Add `@shotershmenimbot` bot to your group and give it admin permissions (it needs to ban users eventually)
* Let users join by send `/join` to the bot in a private message
* When the user joins, they can send a photo/video to the group to report a workout (works once every 30 minutes)
* Use `/help` **only in private** message to the bot to see other options

#### User options
* `/join` - welcomes new users and asks the admin(s) to approve and pick their group. Existing users who were banned will require approval from the admin upon which they'll be send a link to join. After rejoining the bot expects two reports immediately or the user is banned again
* `/status` - tells the user how much time he has left till the end of the 5 days period
