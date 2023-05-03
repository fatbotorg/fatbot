# Fat Bot
The Telegram bot that keeps you lean
---
This is the open sourced code for the Telegram bot that manages groups of users helping them workout.

### Getting Started
* Add `@shotershmenimbot` bot to your group and give it admin permissions (it needs to ban users eventually)
* Send a photo/video to the group to report a workout (works once every 30 minutes)
* Use `/help` **only in private** message to the bot to see other options

#### User options
* `/join` - welcomes new users and asks the admin(s) to approve and pick their group. Existing users who were banned will require approval from the admin upon which they'll be send a link to join. After rejoining the bot expects two reports immediately or the user is banned again
* `/status` - tells the user how much time he has left till the end of the 5 days period
>>>>>>> 91fc03b (readme WIP)

### Features
* Last day notification
* Ban user on day 0
