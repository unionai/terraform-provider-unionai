package provider

var (
	DefaultLimit  int32 = 100
	DefaultFilter       = Filters{
		Limit:  DefaultLimit,
		SortBy: "created_at",
		Asc:    false,
	}
)

type Filters struct {
	FieldSelector string `json:"fieldSelector" pflag:",Allows for filtering resources based on a specific value for a field name using operations =,!=,>,<,>=,<=,in,contains.Multiple selectors can be added separated by commas"`
	SortBy        string `json:"sortBy" pflag:",Specifies which field to sort results "`
	Limit         int32  `json:"limit" pflag:",Specifies the number of results to return"`
	Asc           bool   `json:"asc"  pflag:",Specifies the sorting order. By default sorts result in descending order"`
	Token         string `json:"token" pflag:",Specifies the server provided token to use for fetching next page in case of multi page result"`
}
