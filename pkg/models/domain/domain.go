package domain

// Domain -- model for available service domain for resource-service db
//
// swagger:model
type Domain struct {
	ID string `json:"_id,omitempty" bson:"_id,omitempty"`
	//Domain address
	// required: true
	Domain string `json:"domain"`
	//Group for domain
	// required: true
	DomainGroup string `json:"domain_group"`
	//Domain ip addresses
	// required: true
	IP []string `json:"ip"`
}

// DomainList -- domains list
//
// swagger:model
type DomainList []Domain
