package commands

import "strings"

// runNotify accepts 1/0 and on/off. Both spellings were asked for, and a switch
// the user has to remember the exact word for is a switch they will get wrong
// on a 20-column screen.
func runNotify(ctx Ctx, args string) string {
	if ctx.Notify == nil {
		return "notify: unavailable here"
	}
	arg := strings.ToLower(strings.TrimSpace(args))
	if arg == "" {
		return "notify: " + onOff(ctx.Notify.NotifyEnabled())
	}
	if arg == "1" || arg == "on" {
		ctx.Notify.SetNotify(true)
		return "notify: on"
	}
	if arg == "0" || arg == "off" {
		ctx.Notify.SetNotify(false)
		return "notify: off"
	}
	return "notify: use 1 or 0"
}

func onOff(v bool) string {
	if v {
		return "on"
	}
	return "off"
}

func init() {
	register("ping", Command{
		Help: "answer Pong!",
		Run:  func(ctx Ctx, args string) string { return "Pong!" },
	})

	register("notify", Command{
		Help: "alerts on/off (1|0)",
		Run:  runNotify,
	})

	register("help", Command{
		Help: "list commands",
		Run:  func(ctx Ctx, args string) string { return List() },
	})
}
