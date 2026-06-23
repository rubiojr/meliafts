package sampledb

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

var me = addr{accountName, accountEmail}

var people = []addr{
	{"Bob Smith", "bob@acme.com"},
	{"Alice Johnson", "alice@example.com"},
	{"Carol Danvers", "carol@work.com"},
	{"Dave Ops", "dave@work.com"},
	{"Erin Park", "erin@startup.io"},
	{"Frank Lee", "frank@acme.com"},
	{"Grace Hopper", "grace@navy.mil"},
}

var newsletters = []addr{
	{"EFFector List", "editor@eff.org"},
	{"Kubernetes Weekly", "hello@kubeweekly.io"},
	{"The Verge", "newsletter@theverge.com"},
	{"Hacker Newsletter", "mail@hackernewsletter.com"},
	{"Golang Weekly", "golang@cooperpress.com"},
}

var services = []addr{
	{"GitHub", "noreply@github.com"},
	{"Stripe", "receipts@stripe.com"},
	{"AWS", "no-reply@amazon.com"},
	{"LinkedIn", "messages-noreply@linkedin.com"},
	{"PayPal", "service@paypal.com"},
}

var spammers = []addr{
	{"Mega Deals", "win@deals-4-u.biz"},
	{"Crypto Riches", "invest@moon-coin.xyz"},
	{"Pharma Plus", "sales@cheap-pillz.example"},
}

type category struct {
	weight    int
	folder    string
	fromMe    bool
	senders   []addr
	subjects  []string
	bodies    []string
	htmlPct   int
	attachPct int
	unreadPct int
}

var categories = []category{
	{
		weight: 34, folder: "f-inbox", senders: people, unreadPct: 30, attachPct: 12,
		subjects: []string{
			"Re: Q3 planning", "Kubernetes rollout plan", "Meeting notes — sprint review",
			"Can you review my PR?", "Lunch on Friday?", "Updated kubernetes manifests",
			"Budget approval needed", "Design doc for the new API", "Standup moved to 10am",
		},
		bodies: []string{
			"Hey, can you take a look when you get a chance? Thanks.",
			"The kubernetes rollout is scheduled for next week across the staging clusters.",
			"Here are the notes from today's sprint review. Action items at the bottom.",
			"I pushed a PR that refactors the scheduler. Would love your review.",
			"Quick reminder about the deployment freeze starting Friday.",
		},
	},
	{
		weight: 18, folder: "f-inbox", senders: newsletters, unreadPct: 60, htmlPct: 70,
		subjects: []string{
			"Privacy Isn't Dead. Far From It.", "This week in Kubernetes",
			"Security roundup: what you missed", "The best of the web this week",
			"Go 1.x is out — here's what's new",
		},
		bodies: []string{
			"This week we cover privacy, encryption, and the fight for a free internet. Read on.",
			"Cluster autoscaling, eBPF, and a deep dive into kubernetes networking.",
			"A roundup of the most important security stories, vulnerabilities and patches.",
		},
	},
	{
		weight: 14, folder: "f-inbox", senders: services, attachPct: 65, unreadPct: 20,
		subjects: []string{
			"Invoice #%d for your subscription", "Your receipt from Stripe",
			"AWS bill is ready", "Payment received — thank you", "Invoice reminder",
		},
		bodies: []string{
			"Thanks for your payment. Your invoice is attached as a PDF for your records.",
			"Your monthly receipt is ready. No action is required.",
			"This is a friendly reminder that invoice is due at the end of the month.",
		},
	},
	{
		weight: 12, folder: "f-inbox", senders: services, unreadPct: 45,
		subjects: []string{
			"You have a new connection request", "Someone mentioned you in a pull request",
			"Your weekly activity digest", "New sign-in to your account",
		},
		bodies: []string{
			"You have new notifications waiting. Sign in to see what's happening.",
			"Someone replied to a thread you're following. Click to view the discussion.",
		},
	},
	{
		weight: 14, folder: "f-sent", fromMe: true, senders: append(append([]addr{}, people...), services[0]),
		subjects: []string{
			"Re: lunch on Friday", "Re: Q3 planning", "Re: can you review my PR?",
			"Sending over the invoice", "Notes from our call", "Re: kubernetes rollout",
		},
		bodies: []string{
			"Sounds good — see you then!", "Thanks, I've reviewed it and left a few comments.",
			"Attaching the invoice as discussed. Let me know if anything looks off.",
			"Great chatting earlier. Here's a summary of what we agreed.",
		},
	},
	{
		weight: 8, folder: "f-spam", senders: spammers, unreadPct: 90,
		subjects: []string{
			"You won a $1000 gift card - claim now!!!", "URGENT: your account needs verification",
			"Crypto is mooning, don't miss out", "Cheap meds, no prescription needed",
		},
		bodies: []string{
			"Congratulations! Click here to claim your prize before it expires.",
			"Your account will be suspended unless you verify immediately.",
		},
	},
}

var totalWeight = func() int {
	w := 0
	for _, c := range categories {
		w += c.weight
	}
	return w
}()

func pickCategory(rng *rand.Rand) category {
	n := rng.Intn(totalWeight)
	for _, c := range categories {
		if n < c.weight {
			return c
		}
		n -= c.weight
	}
	return categories[0]
}

func pick[T any](rng *rand.Rand, s []T) T { return s[rng.Intn(len(s))] }

func randomMessage(rng *rand.Rand, now time.Time) message {
	c := pickCategory(rng)
	other := pick(rng, c.senders)

	subject := pick(rng, c.subjects)
	if strings.Contains(subject, "%d") {
		subject = fmt.Sprintf(subject, 1000+rng.Intn(9000))
	}
	body := pick(rng, c.bodies)

	m := message{
		folderID: c.folder,
		subject:  maybeReply(rng, subject),
		body:     body,
		date:     randDate(rng, now, 200),
		flagged:  rng.Intn(100) < 8,
		attach:   rng.Intn(100) < c.attachPct,
	}
	if c.fromMe {
		m.from, m.to, m.read = me, []addr{other}, true
		m.draft = c.folder == "f-drafts"
	} else {
		m.from, m.to = other, []addr{me}
		m.read = rng.Intn(100) >= c.unreadPct
	}
	if rng.Intn(100) < c.htmlPct {
		m.bodyHTML = htmlWrap(m.subject, body)
	}
	return m
}

func maybeReply(rng *rand.Rand, subject string) string {
	switch rng.Intn(10) {
	case 0:
		return "Re: " + subject
	case 1:
		return "Fwd: " + subject
	default:
		return subject
	}
}

func randDate(rng *rand.Rand, now time.Time, maxDays int) time.Time {
	f := rng.Float64()
	days := int(f * f * float64(maxDays)) // bias toward recent
	d := now.AddDate(0, 0, -days)
	d = d.Add(-time.Duration(rng.Intn(24)) * time.Hour)
	d = d.Add(-time.Duration(rng.Intn(60)) * time.Minute)
	return d
}

func htmlWrap(subject, body string) string {
	return "<html><body><h1>" + subject + "</h1><p>" + body +
		"</p><p>You are receiving this because you subscribed. " +
		"<a href=\"https://example.com/unsubscribe\">Unsubscribe</a>.</p></body></html>"
}

// curatedMessages are a few recent, demo-friendly messages that make the
// example queries in the README return something interesting.
func curatedMessages(now time.Time) []message {
	day := func(d int) time.Time { return now.AddDate(0, 0, -d).Add(-3 * time.Hour) }
	return []message{
		{
			folderID: "f-inbox", from: newsletters[0], to: []addr{me},
			subject:  "Privacy Isn't Dead. Far From It. | EFFector 36.3",
			body:     "This is a friendly message from the Electronic Frontier Foundation. Privacy isn't dead — far from it. In this issue we cover surveillance, encryption and your digital rights.",
			bodyHTML: htmlWrap("EFFector 36.3", "Privacy isn't dead — far from it."),
			date:     day(2),
		},
		{
			folderID: "f-inbox", from: newsletters[1], to: []addr{me},
			subject: "Kubernetes Weekly: cluster autoscaling deep-dive",
			body:    "This week: a deep dive into kubernetes cluster autoscaling, plus news on eBPF, service meshes and the latest release.",
			date:    day(1),
		},
		{
			folderID: "f-inbox", from: people[0], to: []addr{me},
			subject: "Invoice #2041 for March consulting",
			body:    "Hi, please find the invoice for March attached. Payment is due within 30 days. Thanks for the kubernetes migration work!",
			date:    day(3), attach: true,
		},
		{
			folderID: "f-inbox", from: people[2], to: []addr{me},
			subject: "Re: Q3 roadmap — your input needed",
			body:    "Could you review the Q3 roadmap before Friday? I flagged the items that need a decision from your team.",
			date:    day(4), flagged: true,
		},
		{
			folderID: "f-sent", from: me, to: []addr{people[1]},
			subject: "Re: lunch on Friday",
			body:    "Friday works for me — let's meet at the usual place at noon.",
			date:    day(1), read: true,
		},
		{
			folderID: "f-spam", from: spammers[1], to: []addr{me},
			subject: "Crypto is mooning, don't miss out",
			body:    "Invest now and triple your money overnight. This opportunity won't last!",
			date:    day(2),
		},
	}
}
