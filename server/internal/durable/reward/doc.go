// Package reward owns the persistent Reward Claim engine
// (reward_claims.md, ADR-0060/0062/0063/0064/0065): typed claim tables,
// contribution-ledger dedupe, consolidation and cap policy, the 408
// claim executor, 434/439/440/441 paging and state surfaces, and the
// JournalRewardCommand kind REST consumer (sim.rest_settlement, 000006
// character_rest_daily). It never imports sim/; producers create claims
// inside their own settlement transactions through Create, and the
// composition ProducerClient family-mux binds "reward.claim".
package reward
