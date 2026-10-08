package auction

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// SearchQuery is the read-side contract of C2S_AUCTION_SEARCH (738):
// item/category/tier/price filters, one sort order, keyset page cursor,
// page size 1..50. Search is read-only and never idempotency-stored.
type SearchQuery struct {
	ItemID    string
	Category  string
	Tier      int
	MinPrice  int64
	MaxPrice  int64
	Sort      SearchSort
	Cursor    string
	PageSize  int
	AccountID id.UUID
}

// SearchSort mirrors the wire AuctionSort enum.
type SearchSort int

const (
	SortPriceAsc SearchSort = iota
	SortPriceDesc
	SortNewest
)

// SearchPage is one keyset page plus the opaque next cursor ("" = last).
type SearchPage struct {
	Listings   []*Listing
	NextCursor string
}

// cursor encodes the last row's keyset tuple; the format is internal and
// opaque to the client (item_id|price|listing_id|listed_at_unix_ms).
func encodeCursor(l *Listing) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%s|%d|%s|%d",
		l.ItemID, l.PriceCommon, l.ListingID.String(), l.ListedAt.UnixMilli())))
}

func decodeCursor(cur string) (itemID string, price int64, listingID string, listedMs int64, err error) {
	b, err := base64.RawURLEncoding.DecodeString(cur)
	if err != nil {
		return "", 0, "", 0, ErrOutOfRange
	}
	parts := strings.Split(string(b), "|")
	if len(parts) != 4 {
		return "", 0, "", 0, ErrOutOfRange
	}
	if _, err = fmt.Sscanf(parts[1], "%d", &price); err != nil {
		return "", 0, "", 0, ErrOutOfRange
	}
	if _, err = fmt.Sscanf(parts[3], "%d", &listedMs); err != nil {
		return "", 0, "", 0, ErrOutOfRange
	}
	return parts[0], price, parts[2], listedMs, nil
}

// Search runs one keyset page over ACTIVE, unexpired listings. The
// canonical index (item_id, price_common, listing_id) WHERE ACTIVE
// supports the order; category/tier resolve through the injected Lookup
// (over-fetch then filter preserves the requested page size).
func (s *Store) Search(ctx context.Context, tx pgx.Tx, q SearchQuery) (*SearchPage, error) {
	if q.PageSize < 1 || q.PageSize > maxPageSize {
		return nil, ErrOutOfRange
	}
	var orderBy, cmp string
	switch q.Sort {
	case SortPriceAsc:
		orderBy = "price_common ASC, listing_id ASC"
		cmp = ">"
	case SortPriceDesc:
		orderBy = "price_common DESC, listing_id ASC"
		cmp = "<"
	case SortNewest:
		orderBy = "listed_at DESC, listing_id ASC"
		cmp = "<"
	default:
		return nil, ErrOutOfRange
	}
	args := []any{q.ItemID, q.MinPrice, q.MaxPrice}
	where := `state='ACTIVE' AND expires_at > now()
	  AND ($1='' OR item_id=$1)
	  AND ($2=0 OR price_common>=$2) AND ($3=0 OR price_common<=$3)`
	if q.Cursor != "" {
		cItem, cPrice, cID, cMs, err := decodeCursor(q.Cursor)
		if err != nil {
			return nil, err
		}
		switch q.Sort {
		case SortNewest:
			where += ` AND (listed_at, listing_id) < (to_timestamp($4/1000.0), $5)`
			args = append(args, cMs, cID)
		default:
			where += fmt.Sprintf(` AND (price_common, listing_id) %s ($4, $5)`, cmp)
			args = append(args, cPrice, cID)
		}
		_ = cItem
	}
	// Over-fetch keeps page_size stable after category/tier filtering.
	fetch := q.PageSize + 1
	if q.Category != "" || q.Tier != 0 {
		fetch = maxPageSize + 1
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT listing_id, seller_character_id,
		seller_account_id, item_instance_id, item_id, quantity, price_common,
		listing_fee_common, state, listed_at, expires_at, ended_at,
		buyer_character_id, revision
	  FROM auction_listings WHERE %s ORDER BY %s LIMIT %d`,
		where, orderBy, fetch), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var all []*Listing
	for rows.Next() {
		l := &Listing{}
		var buyer *string
		if err := rows.Scan(&l.ListingID, &l.SellerCharacterID, &l.SellerAccountID,
			&l.ItemInstanceID, &l.ItemID, &l.Quantity, &l.PriceCommon,
			&l.ListingFeeCommon, &l.State, &l.ListedAt, &l.ExpiresAt,
			&l.EndedAt, &buyer, &l.Revision); err != nil {
			return nil, err
		}
		if buyer != nil {
			if b, perr := id.ParseUUID(*buyer); perr == nil {
				l.BuyerCharacterID = &b
			}
		}
		all = append(all, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var page []*Listing
	for _, l := range all {
		if len(page) == q.PageSize {
			break
		}
		if q.Category != "" || q.Tier != 0 {
			lk, err := s.defs(ctx, l.ItemID)
			if err != nil {
				return nil, err
			}
			if q.Category != "" && lk.Category != q.Category {
				continue
			}
			if q.Tier != 0 && lk.Tier != q.Tier {
				continue
			}
		}
		page = append(page, l)
	}
	out := &SearchPage{Listings: page}
	if len(page) == q.PageSize && (len(all) > len(page) || len(all) == fetch) {
		out.NextCursor = encodeCursor(page[len(page)-1])
	}
	return out, nil
}
