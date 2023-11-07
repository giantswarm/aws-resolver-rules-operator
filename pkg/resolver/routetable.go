package resolver

type RouteTable struct {
	RouteTableId *string
	RouteRules   []RouteRule
}

type RouteRule struct {
	DestinationPrefixListId *string
	TransitGatewayId        *string
}
