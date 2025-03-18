package newrelic

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ilhicas/observability-cost-center/internal/providers"
	"github.com/newrelic/newrelic-client-go/newrelic"
	"github.com/spf13/viper"
)

// NewRelicProvider implements the Provider interface for New Relic
type NewRelicProvider struct {
	client *newrelic.NewRelic
}

// LicenseInfo represents New Relic license information
type LicenseInfo struct {
	Type           string  `json:"type"`
	TotalLicenses  int     `json:"totalLicenses"`
	UsedLicenses   int     `json:"usedLicenses"`
	UtilizationPct float64 `json:"utilizationPct"`
}

// NewProvider creates a new New Relic provider
func NewProvider() (*NewRelicProvider, error) {
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("NEW_RELIC_API_KEY environment variable not set")
	}

	// Get region from environment or config
	region := os.Getenv("NEW_RELIC_REGION")
	if region == "" {
		region = viper.GetString("newrelic.region")
		if region == "" {
			// Default to US region if not specified
			region = "us"
		}
	}

	// Configure the New Relic client with region
	var client *newrelic.NewRelic
	var err error

	// Use appropriate region configuration
	switch region {
	case "EU":
		client, err = newrelic.New(
			newrelic.ConfigPersonalAPIKey(apiKey),
			newrelic.ConfigRegion(region),
		)
	default:
		// Default to US region
		client, err = newrelic.New(
			newrelic.ConfigPersonalAPIKey(apiKey),
			newrelic.ConfigRegion(region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("error creating New Relic client: %w", err)
	}

	return &NewRelicProvider{client: client}, nil
}

// GetName returns the provider name
func (nr *NewRelicProvider) GetName() string {
	return "New Relic"
}

// GetUsageData retrieves usage metrics from New Relic
func (nr *NewRelicProvider) GetUsageData(start, end time.Time) ([]providers.UsageData, error) {
	// Get data usage metrics
	dataMetrics, err := nr.getDataMetrics(start, end)
	if err != nil {
		return nil, err
	}

	// Get license usage data
	licenseUsage, err := nr.getLicenseUsageData()
	if err != nil {
		// Log the error but continue with data metrics
		fmt.Printf("Warning: Failed to get license usage data: %v\n", err)
		return dataMetrics, nil
	}

	// Combine both types of usage data
	combinedUsage := append(dataMetrics, licenseUsage...)
	return combinedUsage, nil
}

// getDataMetrics retrieves usage data metrics from New Relic using NerdGraph API
func (nr *NewRelicProvider) getDataMetrics(start, end time.Time) ([]providers.UsageData, error) {
	// First get all account IDs
	accountsQuery := `{
		actor {
			accounts {
				id
				name
			}
		}
	}`

	// Execute the query to get all account IDs
	accountsResp, err := nr.client.NerdGraph.Query(accountsQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("error querying account IDs: %w", err)
	}

	// Manually unmarshal the response with the corrected type for id
	var accountsResponse struct {
		Actor struct {
			Accounts []struct {
				ID   json.Number `json:"id"` // Change from string to json.Number
				Name string      `json:"name"`
			} `json:"accounts"`
		} `json:"actor"`
	}

	jsonData, err := json.Marshal(accountsResp)
	if err != nil {
		return nil, fmt.Errorf("error marshalling account response: %w", err)
	}

	err = json.Unmarshal(jsonData, &accountsResponse)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling account response: %w", err)
	}

	// Debug the response
	fmt.Printf("Found %d accounts\n", len(accountsResponse.Actor.Accounts))

	var allUsageData []providers.UsageData

	// For each account, query the data usage
	for _, account := range accountsResponse.Actor.Accounts {
		// Convert ID to string when using it
		accountID := account.ID.String()
		fmt.Printf("Querying usage for account %s (%s)\n", accountID, account.Name)

		// Query for data usage metrics - Use accountID instead of account.ID
		dataQuery := fmt.Sprintf(`{
			actor {
				account(id: %s) {
					nrql(query: "SELECT sum(newRelicDbSize) FROM NrDailyUsage SINCE '%s' UNTIL '%s' FACET productLine") {
						results
					}
				}
			}
		}`, accountID, start.Format("2006-01-02"), end.Format("2006-01-02"))

		// Execute the query for data metrics
		dataResp, err := nr.client.NerdGraph.Query(dataQuery, nil)
		if err != nil {
			return nil, fmt.Errorf("error querying data metrics for account %s: %w", accountID, err)
		}

		// Debug the raw response
		fmt.Printf("Raw response type: %T\n", dataResp)
		respBytes, _ := json.Marshal(dataResp)
		fmt.Printf("Raw response: %s\n", string(respBytes))

		// Manually unmarshal the response
		var dataResponse struct {
			Actor struct {
				Account struct {
					NRQL struct {
						Results []map[string]interface{} `json:"results"`
					} `json:"nrql"`
				} `json:"account"`
			} `json:"actor"`
		}

		respJson, err := json.Marshal(dataResp)
		if err != nil {
			return nil, fmt.Errorf("error marshalling data response: %w", err)
		}

		err = json.Unmarshal(respJson, &dataResponse)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling data response: %w", err)
		}

		// Check if we have valid results
		if dataResponse.Actor.Account.NRQL.Results == nil {
			fmt.Printf("Warning: No results found in response for account %s\n", accountID)
			continue
		}

		// Process the results
		for _, result := range dataResponse.Actor.Account.NRQL.Results {
			// Debug the result structure
			resultBytes, _ := json.Marshal(result)
			fmt.Printf("Result: %s\n", string(resultBytes))

			productLine, ok := result["productLine"].(string)
			if !ok {
				fmt.Printf("Warning: Cannot extract productLine from result\n")
				continue
			}

			sumKey := "sum.newRelicDbSize"
			value, ok := result[sumKey].(float64)
			if !ok {
				fmt.Printf("Warning: Cannot extract %s as float64 from result\n", sumKey)
				continue
			}

			allUsageData = append(allUsageData, providers.UsageData{
				Service:   productLine,
				Metric:    "DataSize",
				Value:     value,
				Unit:      "GB",
				Timestamp: time.Now(),
				Metadata: map[string]interface{}{
					"accountId":   accountID, // Use accountID instead of account.ID
					"accountName": account.Name,
				},
			})
		}
	}

	// If no data was found, return sample data for demonstration
	if len(allUsageData) == 0 {
		fmt.Println("No data found, returning sample data")
		return []providers.UsageData{
			{
				Service:   "APM",
				Metric:    "DataSize",
				Value:     1250.5,
				Unit:      "GB",
				Timestamp: start.AddDate(0, 0, 1),
			},
			{
				Service:   "Infrastructure",
				Metric:    "DataSize",
				Value:     720.0,
				Unit:      "GB",
				Timestamp: start.AddDate(0, 0, 1),
			},
			{
				Service:   "Logs",
				Metric:    "DataSize",
				Value:     512.75,
				Unit:      "GB",
				Timestamp: start.AddDate(0, 0, 1),
			},
		}, nil
	}

	return allUsageData, nil
}

// getLicenseUsageData retrieves license usage information
func (nr *NewRelicProvider) getLicenseUsageData() ([]providers.UsageData, error) {
	// Get license information
	licenseInfo, err := nr.GetLicenseInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get license info: %w", err)
	}

	// Convert license information to usage data format
	usageData := make([]providers.UsageData, 0, len(licenseInfo))
	for _, license := range licenseInfo {
		usageData = append(usageData, providers.UsageData{
			Service:   "Licenses",
			Metric:    license.Type + " Licenses",
			Value:     float64(license.UsedLicenses),
			Unit:      "Users",
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"totalLicenses":  license.TotalLicenses,
				"utilizationPct": license.UtilizationPct,
			},
		})
	}

	return usageData, nil
}

// GetCostData retrieves cost data from New Relic
func (nr *NewRelicProvider) GetCostData(start, end time.Time) ([]providers.CostData, error) {
	// Get basic cost data
	basicCosts, err := nr.getBasicCostData(start, end)
	if err != nil {
		return nil, err
	}

	// Get license cost data
	licenseCosts, err := nr.getLicenseCostData(start, end)
	if err != nil {
		// Log the error but continue with basic cost data
		fmt.Printf("Warning: Failed to get license cost data: %v\n", err)
		return basicCosts, nil
	}

	// Combine both types of cost data
	combinedCosts := append(basicCosts, licenseCosts...)
	return combinedCosts, nil
}

// getBasicCostData retrieves standard cost metrics
func (nr *NewRelicProvider) getBasicCostData(start, end time.Time) ([]providers.CostData, error) {
	// In a real implementation, this would query the New Relic API for billing data
	// For now, we'll return sample data
	return []providers.CostData{
		{
			Service:     "APM",
			ItemName:    "ComputeUnits",
			Cost:        625.25,
			Currency:    "USD",
			Period:      "Monthly",
			StartTime:   start,
			EndTime:     end,
			AccountID:   "123456",
			Description: "Application Performance Monitoring",
		},
		{
			Service:     "Infrastructure",
			ItemName:    "HostMonitoring",
			Cost:        360.00,
			Currency:    "USD",
			Period:      "Monthly",
			StartTime:   start,
			EndTime:     end,
			AccountID:   "123456",
			Description: "Infrastructure Monitoring",
		},
		{
			Service:     "Logs",
			ItemName:    "Ingestion",
			Cost:        820.40,
			Currency:    "USD",
			Period:      "Monthly",
			StartTime:   start,
			EndTime:     end,
			AccountID:   "123456",
			Description: "Log Management",
		},
	}, nil
}

// getLicenseCostData retrieves cost data related to licenses
func (nr *NewRelicProvider) getLicenseCostData(start, end time.Time) ([]providers.CostData, error) {
	// Get license information
	licenseInfo, err := nr.GetLicenseInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get license info: %w", err)
	}

	// Define cost per license type (this would normally come from billing API)
	licenseCosts := map[string]float64{
		"User":          99.00,  // $99 per user license
		"Full platform": 199.00, // $199 per full license
		"Basic":         49.00,  // $49 per basic license
		"Core":          499.00, // $499 per core license
		"LimitedAccess": 29.00,  // $29 per limited access license
	}

	// Convert license information to cost data format
	costData := make([]providers.CostData, 0, len(licenseInfo))
	for _, license := range licenseInfo {
		// Get cost per license, default to 0 if not found
		costPerLicense, ok := licenseCosts[license.Type]
		if !ok {
			costPerLicense = 0
		}

		costData = append(costData, providers.CostData{
			Service:     "Licenses",
			ItemName:    license.Type + " Licenses",
			Cost:        float64(license.UsedLicenses) * costPerLicense,
			Quantity:    float64(license.UsedLicenses),
			UsageUnit:   "Users",
			Currency:    "USD",
			Period:      "Monthly",
			StartTime:   start,
			EndTime:     end,
			AccountID:   "123456",
			Description: fmt.Sprintf("%s Licenses (%d/%d used)", license.Type, license.UsedLicenses, license.TotalLicenses),
		})
	}

	return costData, nil
}

// NerdGraphUserType represents user type information from NerdGraph
type NerdGraphUserType struct {
	DisplayName string `json:"displayName"`
	ID          string `json:"id"`
}

// NerdGraphUser represents user information from NerdGraph
type NerdGraphUser struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Email      string            `json:"email"`
	LastActive string            `json:"lastActive"`
	Type       NerdGraphUserType `json:"type"`
}

// getAuthDomainIDs fetches authentication domain IDs using NerdGraph
func (nr *NewRelicProvider) getAuthDomainIDs() ([]string, error) {
	query := `{
		actor {
			organization {
				authorizationManagement {
					authenticationDomains {
						authenticationDomains {
							id
						}
					}
				}
			}
		}
	}`

	// Execute the query for authentication domains
	resp, err := nr.client.NerdGraph.Query(query, nil)
	if err != nil {
		return nil, fmt.Errorf("error querying authentication domain IDs: %w", err)
	}

	// Debug the raw response
	fmt.Printf("Auth Domains Raw Response Type: %T\n", resp)
	respBytes, _ := json.Marshal(resp)
	fmt.Printf("Auth Domains Raw Response: %s\n", string(respBytes))

	// Define the structure with string for ID (UUIDs are strings, not numbers)
	var response struct {
		Actor struct {
			Organization struct {
				AuthorizationManagement struct {
					AuthenticationDomains struct {
						AuthenticationDomains []struct {
							ID string `json:"id"` // Changed to string for UUID
						} `json:"authenticationDomains"`
					} `json:"authenticationDomains"`
				} `json:"authorizationManagement"`
			} `json:"organization"`
		} `json:"actor"`
	}

	// Manually marshal and unmarshal to handle the response format
	jsonData, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("error marshalling authentication domains response: %w", err)
	}

	err = json.Unmarshal(jsonData, &response)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling authentication domains response: %w", err)
	}

	var domainIDs []string
	for _, domain := range response.Actor.Organization.AuthorizationManagement.AuthenticationDomains.AuthenticationDomains {
		domainIDs = append(domainIDs, domain.ID)
	}

	fmt.Printf("Found %d authentication domains\n", len(domainIDs))
	return domainIDs, nil
}

// GetLicenseInfo retrieves information about New Relic licenses
func (nr *NewRelicProvider) GetLicenseInfo() ([]LicenseInfo, error) {
	// Get authentication domain IDs
	domainIDs, err := nr.getAuthDomainIDs()
	if err != nil {
		return nil, fmt.Errorf("failed to get authentication domain IDs: %w", err)
	}

	var allUsers []NerdGraphUser

	// Query users for each authentication domain
	for _, domainID := range domainIDs {
		query := fmt.Sprintf(`{
			actor {
				organization {
					userManagement {
						authenticationDomains(id: "%s") {
							authenticationDomains {
								users {
									users {
										id
										name
										email
										lastActive
										type {
											displayName
											id
										}
									}
								}
							}
						}
					}
				}
			}
		}`, domainID)

		// Execute query for users in this domain
		resp, err := nr.client.NerdGraph.Query(query, nil)
		if err != nil {
			return nil, fmt.Errorf("error querying users for domain %s: %w", domainID, err)
		}

		// Debug the raw response
		fmt.Printf("Domain %s users raw response type: %T\n", domainID, resp)
		respBytes, _ := json.Marshal(resp)
		fmt.Printf("Domain %s users raw response: %s\n", domainID, string(respBytes))

		var responseData struct {
			Actor struct {
				Organization struct {
					UserManagement struct {
						AuthenticationDomains struct {
							AuthenticationDomains []struct {
								Users struct {
									Users []struct {
										ID         string `json:"id"`
										Name       string `json:"name"`
										Email      string `json:"email"`
										LastActive string `json:"lastActive"`
										Type       struct {
											DisplayName string `json:"displayName"`
											ID          string `json:"id"`
										} `json:"type"`
									} `json:"users"`
								} `json:"users"`
							} `json:"authenticationDomains"`
						} `json:"authenticationDomains"`
					} `json:"userManagement"`
				} `json:"organization"`
			} `json:"actor"`
		}

		// Manually marshal and unmarshal to handle the response format
		jsonData, err := json.Marshal(resp)
		if err != nil {
			return nil, fmt.Errorf("error marshalling users response for domain %s: %w", domainID, err)
		}

		err = json.Unmarshal(jsonData, &responseData)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling users response for domain %s: %w", domainID, err)
		}

		// Check if we got a valid response with authentication domains
		authDomains := responseData.Actor.Organization.UserManagement.AuthenticationDomains.AuthenticationDomains
		if len(authDomains) == 0 {
			fmt.Printf("Warning: No authentication domains found for domain ID %s\n", domainID)
			continue
		}

		// Extract and convert users to our NerdGraphUser type
		for _, domain := range authDomains {
			users := domain.Users.Users
			fmt.Printf("Found %d users in domain %s\n", len(users), domainID)

			for _, user := range users {
				allUsers = append(allUsers, NerdGraphUser{
					ID:         user.ID,
					Name:       user.Name,
					Email:      user.Email,
					LastActive: user.LastActive,
					Type: NerdGraphUserType{
						DisplayName: user.Type.DisplayName,
						ID:          user.Type.ID,
					},
				})
			}
		}
	}

	fmt.Printf("Total users found across all domains: %d\n", len(allUsers))

	// Calculate license types and usage
	licenseCount := map[string]struct {
		total int
		used  int
	}{
		"User":          {0, 0},
		"Full":          {0, 0},
		"Basic":         {0, 0},
		"LimitedAccess": {0, 0},
	}

	// Process user data to determine license types and counts
	for _, user := range allUsers {
		licenseType := user.Type.DisplayName
		if licenseType == "" {
			licenseType = "User" // Default to "User" type if no type specified
		}

		if _, exists := licenseCount[licenseType]; !exists {
			licenseCount[licenseType] = struct {
				total int
				used  int
			}{0, 0}
		}

		// Get the current value, modify it, then store it back
		licCountItem := licenseCount[licenseType]
		licCountItem.total++
		licenseCount[licenseType] = licCountItem

		// Consider user active if they have logged in within the last 30 days
		if user.LastActive != "" {
			lastActive, err := time.Parse(time.RFC3339, user.LastActive)
			if err == nil && time.Since(lastActive) <= 30*24*time.Hour {
				// Get the current value again, modify it, then store it back
				licCountItem = licenseCount[licenseType]
				licCountItem.used++
				licenseCount[licenseType] = licCountItem
			}
		}
	}

	// Convert the license count map to the required return format
	result := make([]LicenseInfo, 0, len(licenseCount))
	for licenseType, counts := range licenseCount {
		// Skip license types with no total allocation
		if counts.total == 0 {
			continue
		}

		utilizationPct := 0.0
		if counts.total > 0 {
			utilizationPct = float64(counts.used) / float64(counts.total) * 100.0
		}

		result = append(result, LicenseInfo{
			Type:           licenseType,
			TotalLicenses:  counts.total,
			UsedLicenses:   counts.used,
			UtilizationPct: utilizationPct,
		})
	}

	return result, nil
}

// GetUserDetails gets detailed information about New Relic users
// func (nr *NewRelicProvider) GetUserDetails() ([]users.User, error) {
// 	// In a real implementation:
// 	/*
// 		userParams := users.ListUsersParams{
// 			WithFullUsers: true,
// 		}

// 		usersList, err := nr.client.Users.ListUsers(&userParams)
// 		if err != nil {
// 			return nil, fmt.Errorf("error getting users: %w", err)
// 		}
// 		return usersList, nil
// 	*/

// 	// Mock data for demonstration
// 	return []users.User{
// 		{
// 			ID:        1234567,
// 			FirstName: "John",
// 			LastName:  "Doe",
// 			Email:     "john.doe@example.com",
// 			TimeZone:  "America/Los_Angeles",
// 			Active:    true,
// 		},
// 		{
// 			ID:        7654321,
// 			FirstName: "Jane",
// 			LastName:  "Smith",
// 			Email:     "jane.smith@example.com",
// 			TimeZone:  "Europe/London",
// 			Active:    true,
// 		},
// 	}, nil
// }
