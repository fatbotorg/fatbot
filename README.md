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

### Creating your own group

Any user can create their own workout accountability group using `/creategroup`. The bot will guide you through the setup:

1. Create a new Telegram group
2. Add the bot to the group
3. Make the bot an admin
4. Toggle "Remain Anonymous" on for the bot, then back off (this converts the group to a supergroup)

The bot handles the rest automatically — registering the group, assigning you as admin, and generating an invite link to share with friends.

* Each user can create one group
* The group creator becomes the local admin
* New members joining get a 5-day grace period before their first workout is due
* If the bot is removed from the group, the group is deactivated and the creator can make a new one

### Usage for admins

Admining a group is done via the admin panel available to you with `/admin` directly to the bot in a DM.

* `Rename User` - easier control of names, lets you set a unique name that is reflected in group messages / report
* `Push Workout` - send a workout that was uploaded late. You can push back in granularity of *days*
* `Delete Workout` - to be used on mistakes / uploads that are not real workouts
* `Show Users` - shows a list of a selected group with participants and their last workout
* `Rejoin User` - allows un-banning a user and sending a join link even if 24 hours since banning have not yet passed
* `Ban User` - bans a user
* `Group Link` - generates a join link that's already sharing the wanted group with the bot, an easier way to join and for the admin to approve
* `Close Group` - permanently shuts down the group (requires typing DELETE to confirm). All members are removed and the group is deactivated.

##### Additional options for superadmins

* `/admin_send_report` shares the weekly report immediately - mainly used for debugging

### Getting started on your own

If you want your own group running on the main server, contact us.
Here's how to run your own:

* Go to bot father on Telegram look for `@BotFather` then `/start` -> `/newbot` -> fill in details and take the API key
* On your machine run `export TELEGRAM_APITOKEN=<token>`
* If you want Open AI's responses to workouts, you'd also want to get one from <https://openai.com> and running `export OPENAI_APITOKEN=<token>`
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

##### Adding users

It's very important that users are only added through the bot and not by users (nor the admin), so that the bot can create them in the system and track them.
In order to do that you have two options:

1. Use the admin panel and generate a group link which you can then share with users
2. Have the user reach out to your bot handle where they'd be asked to run `/start`

In both cases, a private message will be sent to approve the request (if through option `1` then by approving, or with option `2` picking a group).
Notifications for approval are sent to local group admins. If no local admins exist, they fall back to superadmins.

#### User options

* `/creategroup` - create your own workout group (guided setup)
* `/join` - welcomes new users and asks the admin(s) to approve and pick their group. Existing users who were banned will require approval from the admin upon which they'll be sent a link to join. After rejoining the bot expects two reports immediately or the user is banned again
* `/status` - tells the user how much time they have left till the end of the 5 days period
* `/stats` - tells the user how many workouts each member of their group has
