package pingdom

import (
	"fmt"
	"net/url"
	"sync"
	"testing"
	"time"
)

type MockPingdomClient struct {
	returnChecksResponse        ChecksResponse
	returnSummaryOutageResponse map[int]SummaryOutageResponse

	getChecksFilters      url.Values
	getCheckSummaryOutage map[int]url.Values
	mutex                 *sync.RWMutex
}

func (mp *MockPingdomClient) GetChecks(filterOptions url.Values) (*ChecksResponse, error) {

	mp.getChecksFilters = filterOptions
	return &mp.returnChecksResponse, nil
}

func (mp *MockPingdomClient) GetCheckSummaryOutage(ID int, filterOptions url.Values) (*SummaryOutageResponse, error) {
	mp.mutex.Lock()
	defer mp.mutex.Unlock()
	summaryOutageResponse := mp.returnSummaryOutageResponse[ID]
	mp.getCheckSummaryOutage[ID] = filterOptions
	return &summaryOutageResponse, nil
}

func MockPingdomManager(checkResponse ChecksResponse, summaryOutageResponse map[int]SummaryOutageResponse) (*Pingdom, *MockPingdomClient) {
	mockClient := &MockPingdomClient{
		returnChecksResponse:        checkResponse,
		returnSummaryOutageResponse: summaryOutageResponse,
		mutex:                       &sync.RWMutex{},
	}

	mockClient.getCheckSummaryOutage = map[int]url.Values{}
	pingdomManager := NewPingdomManager(mockClient)
	return pingdomManager, mockClient
}

func TestGetAlertByTags(t *testing.T) {

	checkResponse := ChecksResponse{
		Checks: []Check{
			{ID: 1, Name: "foo", Hostname: "foo.com"},
			{ID: 2, Name: "foo2", Hostname: "foo2.com"},
		},
	}

	summaryOutageResponse := map[int]SummaryOutageResponse{
		1: {
			Summary: States{
				States: []State{
					{Status: "up"},
				},
			},
		},
		2: {
			Summary: States{
				States: []State{
					{Status: "up", TimeFrom: 1, TimeTo: 2},
					{Status: "down", TimeFrom: 1, TimeTo: 2},
				},
			},
		},
	}
	pingdomManager, mockClient := MockPingdomManager(checkResponse, summaryOutageResponse)

	from := time.Date(2020, time.January, 01, 0, 0, 0, 0, time.UTC)
	to := time.Date(2020, time.January, 02, 0, 0, 0, 0, time.UTC)
	pingdomChecks, err := pingdomManager.GetAlertByTags("foo,foo2", from, to)

	t.Run("validate checks", func(t *testing.T) {
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(pingdomChecks) != 2 {
			t.Fatalf("unexpected checks length, got %d expected %d", len(pingdomChecks), 2)
		}

		check := pingdomChecks[0]
		if check.ID != 2 {
			t.Fatalf("unexpected check ID, got %d expected %d", check.ID, 2)
		}

		if check.URL != fmt.Sprintf(PageURL, check.ID) {
			t.Fatalf("unexpected check URL, got %s expected %s", check.URL, fmt.Sprintf(PageURL, check.ID))
		}

		if check.Name != "foo2" {
			t.Fatalf("unexpected check name, got %s expected %s", check.Name, "foo2")
		}

		if len(check.Periods) != 2 {
			t.Fatalf("unexpected periods check count, got %d expected %d", len(check.Periods), 2)
		}

		if check.Periods[0].Status != "up" {
			t.Fatalf("unexpected period status, got %s expected %s", check.Periods[0].Status, "up")
		}

		if check.Periods[0].StartUnix != 1 {
			t.Fatalf("unexpected period start time, got %d expected %d", check.Periods[0].StartUnix, 1)
		}

		if check.Periods[0].EndUnix != 2 {
			t.Fatalf("unexpected period end time, got %d expected %d", check.Periods[0].EndUnix, 2)
		}
	})

	t.Run("validate checks request queries", func(t *testing.T) {

		if len(mockClient.getChecksFilters["tags"]) != 1 {
			t.Fatalf("unexpected tags filter query, got %d expected %d", len(mockClient.getChecksFilters["tags"]), 1)
		}

		if mockClient.getChecksFilters["tags"][0] != "foo,foo2" {
			t.Fatalf("unexpected tags filter query value, got %s expected %s", mockClient.getChecksFilters["tags"][0], "foo,foo2")
		}

	})

	t.Run("validate summary.outage request queries", func(t *testing.T) {

		if len(mockClient.getCheckSummaryOutage) != 2 {
			t.Fatalf("unexpected summary outage http requests length, got %d expected %d", len(mockClient.getCheckSummaryOutage), 2)
		}

		checkSummary := mockClient.getCheckSummaryOutage[2]
		if checkSummary["from"][0] != "1577836800" {
			t.Fatalf("unexpected from filter query, got %s expected %s", checkSummary["from"][0], "1577836800")
		}
		if checkSummary["to"][0] != "1577923200" {
			t.Fatalf("unexpected to filter query, got %s expected %s", checkSummary["from"][0], "1577923200")
		}
	})

}
