package bandwagonhost

type ServiceInfo struct {
	VmType                          string         `json:"vm_type"`
	Hostname                        string         `json:"hostname"`
	NodeAlias                       string         `json:"node_alias"`
	NodeLocationId                  string         `json:"node_location_id"`
	NodeLocation                    string         `json:"node_location"`
	NodeDatacenter                  string         `json:"node_datacenter"`
	LocationIpv6Ready               bool           `json:"location_ipv6_ready"`
	Plan                            string         `json:"plan"`
	PlanMonthlyData                 int64          `json:"plan_monthly_data"`
	MonthlyDataMultiplier           int64          `json:"monthly_data_multiplier"`
	PlanDisk                        int64          `json:"plan_disk"`
	PlanRam                         int64          `json:"plan_ram"`
	PlanSwap                        int64          `json:"plan_swap"`
	PlanMaxIpv6s                    int64          `json:"plan_max_ipv6s"`
	Os                              string         `json:"os"`
	Email                           string         `json:"email"`
	DataCounter                     int64          `json:"data_counter"`
	DataNextReset                   int64          `json:"data_next_reset"`
	IpAddresses                     []string       `json:"ip_addresses"`
	PrivateIpAddresses              []any          `json:"private_ip_addresses"`
	IpNullroutes                    []any          `json:"ip_nullroutes"`
	Iso1                            any            `json:"iso1"`
	Iso2                            any            `json:"iso2"`
	AvailableIsos                   []string       `json:"available_isos"`
	PlanPrivateNetworkAvailable     bool           `json:"plan_private_network_available"`
	LocationPrivateNetworkAvailable bool           `json:"location_private_network_available"`
	RdnsAPIAvailable                bool           `json:"rdns_api_available"`
	Ptr                             map[string]any `json:"ptr"`
	Suspended                       bool           `json:"suspended"`
	PolicyViolation                 bool           `json:"policy_violation"`
	SuspensionCount                 any            `json:"suspension_count"`
	TotalAbusePoints                int64          `json:"total_abuse_points"`
	MaxAbusePoints                  int64          `json:"max_abuse_points"`
	FreeIpReplacementInterval       int64          `json:"free_ip_replacement_interval"`
	Error                           int64          `json:"error"`
}
