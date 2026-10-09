// Package social owns the durable side of friends, friend requests,
// blocks and player reports (data_model.md § Social/Party, social.md):
// the committing transactions behind the client.61x durable families
// and the read projections the global service pushes as 616/619.
//
// Chat send (600) is not durable — the ephemeral global runtime fans it
// out (save_rules.md § Closed Durable Queue Producer Registry assigns
// chat.log to IMP-094); only the report path here reads chat_messages
// for the optional evidence reference.
package social
