---
title: Introduction
description: Learn about Alita Robot, a powerful Telegram group management bot built with Go.
---

# What is Alita Robot?

Alita Robot is a modern, feature-rich Telegram group management bot built with Go and the [gotgbot](https://github.com/PaulSonOfLars/gotgbot) library. It provides comprehensive tools for managing Telegram groups of any size.

## Key Features

### Moderation Tools
- **Banning & Muting** - Ban, mute, kick users with optional reasons and time limits
- **Warning System** - Configurable warning limits with automatic actions
- **Anti-Flood Protection** - Prevent message spam with customizable limits
- **Blacklist** - Automatically moderate messages containing blacklisted words

### Content Management
- **Notes** - Save and retrieve text/media snippets with keywords
- **Filters** - Auto-respond to keywords with custom messages
- **Rules** - Set and display group rules
- **Pins** - Advanced message pinning with anti-channel-pin protection

### Automation
- **Welcome Messages** - Customizable greetings for new members
- **CAPTCHA Verification** - Math or text-based verification for new users
- **Auto-Approve** - Automatically approve join requests

### Utilities
- **Multi-Language** - Supports English and Spanish (extensible)
- **Remote Management** - Manage groups from private messages
- **Command Disabling** - Disable specific commands per group

## Architecture Highlights

- **High Performance** - Built in Go with concurrent processing
- **Scalable Caching** - Redis-based caching with stampede protection
- **PostgreSQL Database** - Robust data storage with automatic migrations
- **Prometheus Metrics** - Built-in observability
- **Docker Ready** - Easy deployment with Docker Compose

## Getting Started

Ready to add Alita to your group? Check out the [Quick Start](/getting-started/quick-start) guide.

Want to host your own instance? See the [Self-Hosting](/self-hosting) documentation.
