package model

const (
	channelDefaultOrder = "COALESCE(priority, 0) desc, COALESCE(weight, 0) desc, id desc"
	channelIDOrder      = "id desc"
)

func ChannelListOrder(idSort bool) string {
	if idSort {
		return channelIDOrder
	}
	return channelDefaultOrder
}
