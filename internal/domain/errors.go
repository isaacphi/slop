package domain

type NoConversationError struct{}

func (e NoConversationError) Error() string {
	return "no previous conversations found"
}

func IsNoConversationError(err error) bool {
	_, ok := err.(NoConversationError)
	return ok
}
