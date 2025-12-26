---
title: Quick Start
description: Add Alita Robot to your Telegram group in under 2 minutes.
---

# Quick Start Guide

Get Alita Robot running in your group in just a few steps.

## Step 1: Add the Bot

1. Open Telegram and search for **@AlitaRobot** (or your self-hosted bot)
2. Click **Start** to begin a conversation
3. Click the **Add to Group** button, or use this link: [Add Alita to Group](https://t.me/AlitaRobot?startgroup=true)
4. Select your group and confirm

## Step 2: Make the Bot Admin

For Alita to function properly, it needs admin permissions:

1. Open your group settings
2. Go to **Administrators**
3. Click **Add Administrator**
4. Select **Alita Robot**
5. Enable these permissions:
   - Delete messages
   - Ban users
   - Invite users via link
   - Pin messages
   - Manage video chats (optional)

## Step 3: Configure Basic Settings

Once Alita is an admin, configure the essentials:

### Set Group Rules
```
/setrules Your group rules here.
Be respectful and follow the guidelines.
```

### Set Welcome Message
```
/setwelcome Welcome {mention}! Please read our /rules before chatting.
```

### Enable CAPTCHA (Recommended)
```
/captcha on
/captchamode math
```

## Step 4: Explore Commands

Use `/help` to see all available commands organized by category.

Popular commands to try:
- `/adminlist` - See all group admins
- `/rules` - Display group rules
- `/notes` - List saved notes
- `/filters` - List active filters
- `/locks` - See current lock settings

## Next Steps

- [Command Reference](/commands) - Full list of all commands
- [Welcome Messages](/commands/greetings) - Customize welcome/goodbye messages
- [Moderation](/commands/moderation) - Learn about bans, mutes, and warnings
- [CAPTCHA](/commands/captcha) - Configure verification settings

## Need Help?

- Use `/help` in the bot for quick reference
- Join the [Support Group](https://t.me/DivideSupport) for assistance
- Check the [Troubleshooting](/self-hosting/troubleshooting) guide for common issues
