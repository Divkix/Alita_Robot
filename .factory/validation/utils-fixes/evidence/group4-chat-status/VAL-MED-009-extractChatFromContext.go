// extractChatFromContext extracts the chat from the context.
// It handles callback queries, regular messages, and MyChatMember updates.
// If chat parameter is already provided (non-nil), it returns it directly.
//
// SAFETY NOTE: This function returns pointers to values within the context struct
// or local variables. Go's escape analysis ensures these are heap-allocated when
// their addresses escape, making the returned pointers valid for the lifetime of
// the context. The caller must ensure the context remains valid while using the
// returned pointer. This pattern is safe because:
//  1. Go's compiler escape analysis moves address-taken variables to the heap
//  2. The gotgbot.Chat struct is a value type that gets copied when assigned
//  3. All returned pointers point to stable memory locations
func extractChatFromContext(ctx *ext.Context, chat *gotgbot.Chat) *gotgbot.Chat {
	if chat != nil {
		return chat
	}
	if ctx.CallbackQuery != nil && ctx.CallbackQuery.Message != nil {
		chatValue := ctx.CallbackQuery.Message.GetChat()
		return &chatValue
	}
	if ctx.Message != nil {
		return &ctx.Message.Chat
	}
	if ctx.MyChatMember != nil {
		return &ctx.MyChatMember.Chat
	}
	return nil
}
