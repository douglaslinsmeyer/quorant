package fin

import (
	"context"
	"regexp"
	"strconv"
)

// Regex patterns for parsing vendor payment terms.
var (
	// discountTermsRE matches "2/10 Net 30" style terms.
	// Group 1: discount percent, Group 2: discount days, Group 3: net days.
	discountTermsRE = regexp.MustCompile(`(?i)(\d+)/(\d+)\s+net\s+(\d+)`)

	// netTermsRE matches "Net 30" style terms (no discount).
	// Group 1: net days.
	netTermsRE = regexp.MustCompile(`(?i)net\s+(\d+)`)
)

const defaultNetDays = 30

// PaymentTerms computes payment terms for a payable by parsing the vendor terms
// string. Supported formats:
//   - "2/10 Net 30" -- 2% discount if paid within 10 days, due in 30 days
//   - "Net 30"      -- due in 30 days, no discount
//   - "Net 60"      -- due in 60 days, no discount
//   - empty/unknown -- defaults to Net 30
func (e *GaapEngine) PaymentTerms(_ context.Context, pc PayableContext) (*PaymentTermsResult, error) {
	terms := pc.VendorTerms

	// Try discount terms first: "2/10 Net 30"
	if m := discountTermsRE.FindStringSubmatch(terms); len(m) == 4 {
		discountPct, _ := strconv.ParseFloat(m[1], 64)
		discountDays, _ := strconv.Atoi(m[2])
		netDays, _ := strconv.Atoi(m[3])

		dueDate := pc.InvoiceDate.AddDate(0, 0, netDays)
		discountDate := pc.InvoiceDate.AddDate(0, 0, discountDays)

		return &PaymentTermsResult{
			DueDate:         dueDate,
			DiscountDate:    &discountDate,
			DiscountPercent: &discountPct,
		}, nil
	}

	// Try simple net terms: "Net 30"
	if m := netTermsRE.FindStringSubmatch(terms); len(m) == 2 {
		netDays, _ := strconv.Atoi(m[1])
		dueDate := pc.InvoiceDate.AddDate(0, 0, netDays)

		return &PaymentTermsResult{
			DueDate: dueDate,
		}, nil
	}

	// Default: Net 30
	dueDate := pc.InvoiceDate.AddDate(0, 0, defaultNetDays)
	return &PaymentTermsResult{
		DueDate: dueDate,
	}, nil
}
