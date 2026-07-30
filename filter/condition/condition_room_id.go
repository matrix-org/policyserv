package condition

import "context"

func RoomId(onlyInRoomId string) Condition {
	return newSimpleCondition(func(ctx context.Context, communityId string, roomId string, senderUserId string) bool {
		return onlyInRoomId == roomId
	})
}
