package newrelic

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
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

// UserLicenseData represents detailed license information for a single user
type UserLicenseData struct {
	UserID      string    `json:"userId"`
	UserName    string    `json:"userName"`
	Email       string    `json:"email"`
	LicenseType string    `json:"licenseType"`
	LastActive  time.Time `json:"lastActive"`
	IsActive    bool      `json:"isActive"`
	Cost        float64   `json:"cost"` // Cost of this license
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
	return "newrelic"
}

// GetUsageData retrieves usage metrics from New Relic
func (nr *NewRelicProvider) GetUsageData(start, end time.Time) ([]providers.UsageData, error) {
	// Get data usage metrics
	dataMetrics, err := nr.getDataMetrics(start, end)
	if err != nil {
		return nil, err
	}

	// Get license usage data - always include license usage for New Relic
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
					"accountId":   accountID,
					"accountName": account.Name,
				},
			})
		}
	}

	// If no real data was found, return empty slice instead of sample data
	if len(allUsageData) == 0 {
		fmt.Println("No usage data found for the specified period")
		return []providers.UsageData{}, nil
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

	// Get license cost data - always include license costs for New Relic
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
				ID   json.Number `json:"id"`
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

	fmt.Printf("Found %d accounts for cost data\n", len(accountsResponse.Actor.Accounts))

	var allCostData []providers.CostData

	// For each account, query the cost data
	for _, account := range accountsResponse.Actor.Accounts {
		accountID := account.ID.String()
		fmt.Printf("Querying cost data for account %s (%s)\n", accountID, account.Name)

		// Query for billing data using NerdGraph
		costQuery := fmt.Sprintf(`{
            actor {
                account(id: %s) {
                    nrql(query: "SELECT latest(totalAmount) AS cost, latest(unit) AS unit, latest(productLine) AS productLine, latest(usageMetric) AS metric FROM NrConsumption SINCE '%s' UNTIL '%s' FACET productLine, usageMetric") {
                        results
                    }
                }
            }
        }`, accountID, start.Format("2006-01-02"), end.Format("2006-01-02"))

		// Execute the query for cost data
		costResp, err := nr.client.NerdGraph.Query(costQuery, nil)
		if err != nil {
			return nil, fmt.Errorf("error querying cost data for account %s: %w", accountID, err)
		}

		// Debug the raw response
		fmt.Printf("Cost data raw response type: %T\n", costResp)
		respBytes, _ := json.Marshal(costResp)
		fmt.Printf("Cost data raw response: %s\n", string(respBytes))

		// Unmarshal the cost data response
		var costResponse struct {
			Actor struct {
				Account struct {
					NRQL struct {
						Results []map[string]interface{} `json:"results"`
					} `json:"nrql"`
				} `json:"account"`
			} `json:"actor"`
		}

		respJson, err := json.Marshal(costResp)
		if err != nil {
			return nil, fmt.Errorf("error marshalling cost response: %w", err)
		}

		err = json.Unmarshal(respJson, &costResponse)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling cost response: %w", err)
		}

		// Check if we have valid results
		if costResponse.Actor.Account.NRQL.Results == nil {
			fmt.Printf("Warning: No cost results found in response for account %s\n", accountID)
			continue
		}

		// Process the results
		for _, result := range costResponse.Actor.Account.NRQL.Results {
			// Extract productLine
			productLine, ok := result["productLine"].(string)
			if !ok {
				fmt.Printf("Warning: Cannot extract productLine from cost result\n")
				continue
			}

			// Extract metric
			metric, ok := result["metric"].(string)
			if !ok {
				metric = "Usage" // Default if not available
			}

			// Extract cost
			cost, ok := result["cost"].(float64)
			if !ok {
				fmt.Printf("Warning: Cannot extract cost as float64 from result\n")
				continue
			}

			// Extract unit
			unit, ok := result["unit"].(string)
			if !ok {
				unit = "Count" // Default if not available
			}

			allCostData = append(allCostData, providers.CostData{
				Service:     productLine,
				ItemName:    metric,
				Cost:        cost,
				UsageUnit:   unit,
				Currency:    "USD", // Assuming USD as default currency
				Period:      "Monthly",
				StartTime:   start,
				EndTime:     end,
				AccountID:   accountID,
				Description: fmt.Sprintf("%s - %s (%s)", productLine, metric, account.Name),
			})
		}
	}

	// If no cost data was found, return empty slice
	if len(allCostData) == 0 {
		fmt.Println("No cost data found for the specified period")
	}

	return allCostData, nil
}

// getLicenseCostData retrieves cost data related to licenses
func (nr *NewRelicProvider) getLicenseCostData(start, end time.Time) ([]providers.CostData, error) {
	// First get all account IDs to associate licenses with accounts
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
		return nil, fmt.Errorf("error querying account IDs for license costs: %w", err)
	}

	// Manually unmarshal the response with the corrected type for id
	var accountsResponse struct {
		Actor struct {
			Accounts []struct {
				ID   json.Number `json:"id"`
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

	// Get real account ID or use "unknown" if no accounts found
	accountID := "unknown"
	accountName := "Unknown Account"
	if len(accountsResponse.Actor.Accounts) > 0 {
		accountID = accountsResponse.Actor.Accounts[0].ID.String()
		accountName = accountsResponse.Actor.Accounts[0].Name
	}

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
			AccountID:   accountID,
			Description: fmt.Sprintf("%s Licenses (%d/%d used) - Account: %s", license.Type, license.UsedLicenses, license.TotalLicenses, accountName),
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

// GetLicenseUsageReport generates a detailed report of license usage including per-user details
func (nr *NewRelicProvider) GetLicenseUsageReport(daysInactive int) (string, error) {
	// Get detailed user license information
	userLicenses, err := nr.GetDetailedLicenseData()
	if err != nil {
		return "", fmt.Errorf("error getting detailed license data: %w", err)
	}

	// Define cost per license type (this would normally come from billing API)
	licenseCosts := map[string]float64{
		"User":          99.00,  // $99 per user license
		"Full platform": 199.00, // $199 per full license
		"Basic":         49.00,  // $49 per basic license
		"Core":          499.00, // $499 per core license
		"LimitedAccess": 29.00,  // $29 per limited access license
	}

	// Calculate inactive threshold
	inactiveThreshold := time.Now().AddDate(0, 0, -daysInactive)

	// Process user data to identify inactive users and potential savings
	var totalCost, potentialSavings float64
	inactiveCount := 0
	totalCount := len(userLicenses)

	for i := range userLicenses {
		// Assign cost to each license
		cost, exists := licenseCosts[userLicenses[i].LicenseType]
		if !exists {
			cost = 0
		}
		userLicenses[i].Cost = cost
		totalCost += cost

		// Check if the user is inactive based on the threshold
		if userLicenses[i].LastActive.Before(inactiveThreshold) {
			userLicenses[i].IsActive = false
			inactiveCount++
			potentialSavings += cost
		} else {
			userLicenses[i].IsActive = true
		}
	}

	// Create the report
	var report strings.Builder
	report.WriteString(fmt.Sprintf("License Usage Report for New Relic\n"))
	report.WriteString(fmt.Sprintf("Date: %s\n\n", time.Now().Format("2006-01-02")))

	report.WriteString(fmt.Sprintf("Summary:\n"))
	report.WriteString(fmt.Sprintf("  Total Users: %d\n", totalCount))
	report.WriteString(fmt.Sprintf("  Active Users: %d\n", totalCount-inactiveCount))
	report.WriteString(fmt.Sprintf("  Inactive Users (>%d days): %d\n", daysInactive, inactiveCount))
	report.WriteString(fmt.Sprintf("  Total License Cost: $%.2f\n", totalCost))
	report.WriteString(fmt.Sprintf("  Potential Monthly Savings: $%.2f\n\n", potentialSavings))

	// License type breakdown
	licenseTypeCounts := make(map[string]int)
	licenseTypeInactiveCounts := make(map[string]int)

	for _, user := range userLicenses {
		licenseTypeCounts[user.LicenseType]++
		if !user.IsActive {
			licenseTypeInactiveCounts[user.LicenseType]++
		}
	}

	report.WriteString("License Type Breakdown:\n")
	report.WriteString("TYPE            | TOTAL | INACTIVE | COST PER LICENSE | POTENTIAL SAVINGS\n")
	report.WriteString("----------------+-------+----------+-----------------+------------------\n")

	for licType, count := range licenseTypeCounts {
		cost := licenseCosts[licType]
		inactive := licenseTypeInactiveCounts[licType]
		savings := float64(inactive) * cost
		report.WriteString(fmt.Sprintf("%-16s| %-5d | %-8d | $%-15.2f| $%.2f\n", licType, count, inactive, cost, savings))
	}
	report.WriteString("\n")

	// Detailed user breakdown
	report.WriteString("Detailed License Usage:\n")
	report.WriteString("USERNAME        | EMAIL                  | LICENSE TYPE    | LAST ACTIVE         | STATUS   | COST\n")
	report.WriteString("----------------+------------------------+----------------+---------------------+----------+--------\n")

	// Sort users by inactive status first, then by license type
	sort.Slice(userLicenses, func(i, j int) bool {
		if userLicenses[i].IsActive != userLicenses[j].IsActive {
			return !userLicenses[i].IsActive // Inactive users first
		}
		return userLicenses[i].LicenseType < userLicenses[j].LicenseType
	})

	for _, user := range userLicenses {
		status := "Active"
		if !user.IsActive {
			status = "Inactive"
		}

		// Truncate long fields for better formatting
		userName := truncateString(user.UserName, 16)
		email := truncateString(user.Email, 22)
		licenseType := truncateString(user.LicenseType, 16)

		report.WriteString(fmt.Sprintf("%-16s| %-22s| %-16s| %-19s | %-8s | $%.2f\n",
			userName, email, licenseType, user.LastActive.Format("2006-01-02 15:04:05"),
			status, user.Cost))
	}

	return report.String(), nil
}

// GetDetailedLicenseData retrieves detailed information about each user license
func (nr *NewRelicProvider) GetDetailedLicenseData() ([]UserLicenseData, error) {
	// Get authentication domain IDs
	domainIDs, err := nr.getAuthDomainIDs()
	if err != nil {
		return nil, fmt.Errorf("failed to get authentication domain IDs: %w", err)
	}

	var allUserData []UserLicenseData

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

		// Extract and convert users to our UserLicenseData type
		for _, domain := range authDomains {
			users := domain.Users.Users
			fmt.Printf("Found %d users in domain %s\n", len(users), domainID)

			for _, user := range users {
				// Parse last active time
				var lastActive time.Time
				if user.LastActive != "" {
					parsedTime, err := time.Parse(time.RFC3339, user.LastActive)
					if err == nil {
						lastActive = parsedTime
					} else {
						// If parsing fails, set to a very old date to mark as inactive
						lastActive = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
					}
				} else {
					// If no last active time provided, set to a very old date
					lastActive = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
				}

				// Default license type if not available
				licenseType := user.Type.DisplayName
				if licenseType == "" {
					licenseType = "User" // Default license type
				}

				allUserData = append(allUserData, UserLicenseData{
					UserID:      user.ID,
					UserName:    user.Name,
					Email:       user.Email,
					LicenseType: licenseType,
					LastActive:  lastActive,
					IsActive:    time.Since(lastActive) <= 30*24*time.Hour, // Active if used in last 30 days
				})
			}
		}
	}

	// If no real users were found, return empty slice instead of sample data
	if len(allUserData) == 0 {
		fmt.Println("No real user data found")
		return []UserLicenseData{}, nil
	}

	return allUserData, nil
}

// Helper function to truncate strings for better report formatting
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength-3] + "..."
}
