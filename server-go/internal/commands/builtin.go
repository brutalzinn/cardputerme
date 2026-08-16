package commands

func init() {
	register("ping", Command{
		Help: "answer Pong!",
		Run:  func(ctx Ctx, args string) string { return "Pong!" },
	})

	register("help", Command{
		Help: "list commands",
		Run:  func(ctx Ctx, args string) string { return List() },
	})
}
