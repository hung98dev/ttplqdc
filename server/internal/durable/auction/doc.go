// Package auction implements the fixed-price world Auction House durable
// layer (trading_auction.md § Auction House, data_model.md § Auction /
// Trade): listings under AUCTION_ESCROW custody, atomic purchase
// settlement with seller-proceeds escrow, cancel/expiry/reclaim, the
// 7-day MOVED_TO_CLAIM sweep and the keyset search + my-state projections.
package auction
