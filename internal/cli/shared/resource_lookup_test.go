package shared

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type iapLookupFixture struct {
	id        string
	productID string
	name      string
}

type subscriptionLookupFixture struct {
	id        string
	productID string
	name      string
}

type sequenceIAPLookupStub struct {
	responses []*asc.InAppPurchasesV2Response
	calls     int
}

func (s *sequenceIAPLookupStub) GetInAppPurchasesV2(_ context.Context, _ string, _ ...asc.IAPOption) (*asc.InAppPurchasesV2Response, error) {
	s.calls++
	if len(s.responses) == 0 {
		return &asc.InAppPurchasesV2Response{}, nil
	}
	idx := s.calls - 1
	if idx >= len(s.responses) {
		idx = len(s.responses) - 1
	}
	if s.responses[idx] == nil {
		return &asc.InAppPurchasesV2Response{}, nil
	}
	return s.responses[idx], nil
}

type sequenceSubscriptionLookupStub struct {
	groupResponses []*asc.SubscriptionGroupsResponse
	groupCalls     int

	subscriptionResponses map[string][]*asc.SubscriptionsResponse
	subscriptionCalls     map[string]int
}

func (s *sequenceSubscriptionLookupStub) GetSubscriptionGroups(_ context.Context, _ string, _ ...asc.SubscriptionGroupsOption) (*asc.SubscriptionGroupsResponse, error) {
	s.groupCalls++
	if len(s.groupResponses) == 0 {
		return &asc.SubscriptionGroupsResponse{}, nil
	}
	idx := s.groupCalls - 1
	if idx >= len(s.groupResponses) {
		idx = len(s.groupResponses) - 1
	}
	if s.groupResponses[idx] == nil {
		return &asc.SubscriptionGroupsResponse{}, nil
	}
	return s.groupResponses[idx], nil
}

func (s *sequenceSubscriptionLookupStub) GetSubscriptions(_ context.Context, groupID string, _ ...asc.SubscriptionsOption) (*asc.SubscriptionsResponse, error) {
	if s.subscriptionCalls == nil {
		s.subscriptionCalls = make(map[string]int)
	}
	s.subscriptionCalls[groupID]++

	responses := s.subscriptionResponses[groupID]
	if len(responses) == 0 {
		return &asc.SubscriptionsResponse{}, nil
	}
	idx := s.subscriptionCalls[groupID] - 1
	if idx >= len(responses) {
		idx = len(responses) - 1
	}
	if responses[idx] == nil {
		return &asc.SubscriptionsResponse{}, nil
	}
	return responses[idx], nil
}

func iapResponse(items ...iapLookupFixture) *asc.InAppPurchasesV2Response {
	resp := &asc.InAppPurchasesV2Response{
		Links: asc.Links{},
		Data:  make([]asc.Resource[asc.InAppPurchaseV2Attributes], 0, len(items)),
	}
	for _, item := range items {
		resp.Data = append(resp.Data, asc.Resource[asc.InAppPurchaseV2Attributes]{
			ID: item.id,
			Attributes: asc.InAppPurchaseV2Attributes{
				ProductID: item.productID,
				Name:      item.name,
			},
		})
	}
	return resp
}

func subscriptionGroupsResponse(next string, groupIDs ...string) *asc.SubscriptionGroupsResponse {
	resp := &asc.SubscriptionGroupsResponse{
		Links: asc.Links{Next: next},
		Data:  make([]asc.Resource[asc.SubscriptionGroupAttributes], 0, len(groupIDs)),
	}
	for _, groupID := range groupIDs {
		resp.Data = append(resp.Data, asc.Resource[asc.SubscriptionGroupAttributes]{
			ID: groupID,
		})
	}
	return resp
}

func subscriptionsResponse(items ...subscriptionLookupFixture) *asc.SubscriptionsResponse {
	resp := &asc.SubscriptionsResponse{
		Links: asc.Links{},
		Data:  make([]asc.Resource[asc.SubscriptionAttributes], 0, len(items)),
	}
	for _, item := range items {
		resp.Data = append(resp.Data, asc.Resource[asc.SubscriptionAttributes]{
			ID: item.id,
			Attributes: asc.SubscriptionAttributes{
				ProductID: item.productID,
				Name:      item.name,
			},
		})
	}
	return resp
}

func TestRequireAppForStableSelector_MissingAppContext(t *testing.T) {
	err := RequireAppForStableSelector("", "com.example.pro", "--iap-id")
	if err == nil {
		t.Fatal("expected usage error")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp usage error, got %v", err)
	}
}

func TestRequireAppForStableSelector_NameShapedSelectorRequiresLookup(t *testing.T) {
	err := RequireAppForStableSelector("", "PLAN_ID", "--iap-id")
	if err == nil {
		t.Fatal("expected usage error for PLAN_ID selector")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp usage error, got %v", err)
	}
}

func TestResolveIAPID_NumericPassthroughSkipsLookup(t *testing.T) {
	stub := &sequenceIAPLookupStub{}

	got, err := ResolveIAPID(context.Background(), stub, "", "123456789")
	if err != nil {
		t.Fatalf("ResolveIAPID() error: %v", err)
	}
	if got != "123456789" {
		t.Fatalf("expected numeric passthrough, got %q", got)
	}
	if stub.calls != 0 {
		t.Fatalf("expected no lookup calls, got %d", stub.calls)
	}
}

func TestResolveIAPID_ResolvesExactProductID(t *testing.T) {
	stub := &sequenceIAPLookupStub{
		responses: []*asc.InAppPurchasesV2Response{
			iapResponse(iapLookupFixture{id: "iap-1", productID: "com.example.pro", name: "Pro"}),
		},
	}

	got, err := ResolveIAPID(context.Background(), stub, "app-1", "com.example.pro")
	if err != nil {
		t.Fatalf("ResolveIAPID() error: %v", err)
	}
	if got != "iap-1" {
		t.Fatalf("expected resolved IAP id iap-1, got %q", got)
	}
}

func TestResolveIAPID_FallsBackToFullScanForExactName(t *testing.T) {
	stub := &sequenceIAPLookupStub{
		responses: []*asc.InAppPurchasesV2Response{
			iapResponse(),
			iapResponse(iapLookupFixture{id: "iap-fuzzy", productID: "com.example.fuzzy", name: "Premium Plus"}),
			iapResponse(iapLookupFixture{id: "iap-exact", productID: "com.example.exact", name: "Premium"}),
		},
	}

	got, err := ResolveIAPID(context.Background(), stub, "app-1", "Premium")
	if err != nil {
		t.Fatalf("ResolveIAPID() error: %v", err)
	}
	if got != "iap-exact" {
		t.Fatalf("expected fallback exact-name IAP id iap-exact, got %q", got)
	}
	if stub.calls != 3 {
		t.Fatalf("expected product, name, and full-scan lookups, got %d calls", stub.calls)
	}
}

func TestResolveIAPID_AmbiguousExactNameFails(t *testing.T) {
	stub := &sequenceIAPLookupStub{
		responses: []*asc.InAppPurchasesV2Response{
			iapResponse(),
			iapResponse(
				iapLookupFixture{id: "iap-1", productID: "com.example.one", name: "Pro"},
				iapLookupFixture{id: "iap-2", productID: "com.example.two", name: "Pro"},
			),
		},
	}

	_, err := ResolveIAPID(context.Background(), stub, "app-1", "Pro")
	if err == nil {
		t.Fatal("expected ambiguous error")
	}
	if !strings.Contains(err.Error(), "Use the explicit ASC ID to disambiguate") {
		t.Fatalf("expected disambiguation guidance, got %v", err)
	}
}

func TestResolveSubscriptionID_DedupesAcrossGroups(t *testing.T) {
	stub := &sequenceSubscriptionLookupStub{
		groupResponses: []*asc.SubscriptionGroupsResponse{
			subscriptionGroupsResponse("", "group-1", "group-2"),
		},
		subscriptionResponses: map[string][]*asc.SubscriptionsResponse{
			"group-1": {subscriptionsResponse(subscriptionLookupFixture{id: "sub-1", productID: "com.example.monthly", name: "Monthly"})},
			"group-2": {subscriptionsResponse(subscriptionLookupFixture{id: "sub-1", productID: "com.example.monthly", name: "Monthly"})},
		},
	}

	got, err := ResolveSubscriptionID(context.Background(), stub, "app-1", "com.example.monthly")
	if err != nil {
		t.Fatalf("ResolveSubscriptionID() error: %v", err)
	}
	if got != "sub-1" {
		t.Fatalf("expected deduped subscription id sub-1, got %q", got)
	}
}

func TestResolveSubscriptionID_FallsBackToFullScanForExactName(t *testing.T) {
	stub := &sequenceSubscriptionLookupStub{
		groupResponses: []*asc.SubscriptionGroupsResponse{
			subscriptionGroupsResponse("", "group-1"),
		},
		subscriptionResponses: map[string][]*asc.SubscriptionsResponse{
			"group-1": {
				subscriptionsResponse(),
				subscriptionsResponse(subscriptionLookupFixture{id: "sub-fuzzy", productID: "com.example.plus", name: "Premium Plus"}),
				subscriptionsResponse(subscriptionLookupFixture{id: "sub-exact", productID: "com.example.premium", name: "Premium"}),
			},
		},
	}

	got, err := ResolveSubscriptionID(context.Background(), stub, "app-1", "Premium")
	if err != nil {
		t.Fatalf("ResolveSubscriptionID() error: %v", err)
	}
	if got != "sub-exact" {
		t.Fatalf("expected fallback exact-name subscription id sub-exact, got %q", got)
	}
}
