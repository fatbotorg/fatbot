# Fat Bot
The Telegram bot that keeps you lean
---

![alt text](http://omer.hamerman.co.s3.amazonaws.com/putin.png)

### Usage as a user
As a participant in a group, you're expected to share a workout-related image once every 5 days (hopefully more often than that).

* After a workout upload you have 60 minutes to share more types of media without it being saved by the bot
* If you want to share something outside this period, use `skip` as the caption for the image/video. You can add other text in the lines below.
* You have 5 days between uploads or the bot bans you from the group
* On the 4th day, the bot sends a notification in the group saying there are 24 hours left
* If you want to see how many days you have left for upload you can send a DM to the bot with `/status`
* After being banned you can go the bot and ask it to `/join`, you'll be automatically sent a join link
* A weekly summary / report is sent over the weekend with users workouts, comparison to previous week, and leaders


### Usage for admins
Admining a group is done via the admin panel available to you with `/admin` directly to the bot in a DM.

* `Rename User` - easier control of names, lets you set a unique name that is reflected in group messages / report
* `Push Workout` - send a workout that was uploaded late. You can push back in granularity of *days*
* `Delete Workout` - to be used on mistakes / uploads that are not real workouts
* `Show Workouts` - shows a list of a selected group with participants and their last workout
* `Show Events` - in beta, shows recent events of a user (ban, weekly leader, 24 hours notification, etc)
* `Rejoin User` - allows un-banning a user and sending a join link even if 24 hours since banning have not yet passed
* `Ban User` - bans a user
* `Group Link` - generates a join link that's already sharing the wanted group with the bot, an easier way to join and for the admin to approve

##### Additional options for admins
* `/newgroup` to set up new groups (described below)
* `/admin_send_report` shares the weekly report immediately - mainly used for debugging


### Getting started on your own 
If you want your own group running on the main server, contact us.
Here's how to run your own:

* Go to bot father on Telegram look for `@BotFather` then `/start` -> `/newbot` -> fill in details and take the API key
* On your machine run `export TELEGRAM_APITOKEN=<token>`
* If you want Open AI's responses to workouts, you'd also want to get one from https://openai.com and running `export OPENAI_APITOKEN=<token>`
* For the admin panel to work you'd want to use Redis and expose it locally, the easiest way is: `redis-server —daemonize yes`
* `go run .`
* Unless changes made, the DB will be created locally -> `./fat.db`


##### Making yourself a superadmin
* Superadmins (as opposed to local group admins) are set by the field `is_admin` (`bool`) in the users group
* You'll want to set yourself as a superadmin, for that, you'll need your Telegarm user ID which you can get that from `@userinfobot`
* Open the DB (sqlite) and run:
```
insert into users(username, name,telegram_user_id,active,is_admin) values(<your_username>,<your_name>,<your_telegram_id>,1,1);
```

##### Creating a group
* Create a new group and add the bot your created earlier
* Set the bot as admin
* Critical step: when you're asked to provde the bot with admin permissions *toggle the option to stay annonymous on*. This changes the group to a "supergroup" which is essential for the bot to operate. You can switch it off right away (recommended)
* Another critical step: make sure users *aren't allowed* to invite other to the group!
* As a superadmin, you can now run `/newgroup` in the group chat and the bot should take care of its creation and let you know it worked in a message

##### Adding users
It's very imporatnt that users are only added through the bot and not by users (nor the admin), so that the bot can create them in the system and track them.
In order to do that you have two options:
1. Use the admin panel and generate a group link which you can then share with users
2. Have the user reachout to your bot handle where they'd be asked to run `/start`

In both cases, a private message will be sent to approve the request (if through option `1` then by approving, or with option `2` picking a group).
Important: soon, notifications for approval will only be sent to local admins to prevent superadmins from overload, so make sure you're a local admin in the group as well, use the admin panel for that.


#### User options
* `/join` - welcomes new users and asks the admin(s) to approve and pick their group. Existing users who were banned will require approval from the admin upon which they'll be send a link to join. After rejoining the bot expects two reports immediately or the user is banned again
* `/status` - tells the user how much time he has left till the end of the 5 days period
