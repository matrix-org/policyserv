package notifiers

// MatrixNotifier - Used to send notifications of activity to Matrix. Note that this accepts a "target"
// which might not be a room ID depending on the implementation.
type MatrixNotifier interface {
	// Send - Sends an `m.notice` message to the given community ID, if possible. The implementation might queue the
	// message for later delivery - the returned error represents a queue failure in this case rather than a delivery
	// failure. Returns a "message ID" for logging purposes.
	Send(communityId string, plainText string, htmlText string) (string, error)
}
