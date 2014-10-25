package libkb

import (
	"fmt"
	"strings"
	"time"
)

type TypedChainLink interface {
	GetRevocations() []*SigId
	insertIntoTable(tab *IdentityTable)
	GetSigId() SigId
	GetArmoredSig() string
	markRevoked(l TypedChainLink)
	ToDebugString() string
	Type() string
	ToDisplayString() string
	IsRevocationIsh() bool
	IsRevoked() bool
	IsActiveKey() bool
	GetSeqno() Seqno
	GetCTime() time.Time
	GetPgpFingerprint() PgpFingerprint
	GetUsername() string
	MarkChecked(ProofError)
}

//=========================================================================
// GenericChainLink
//

type GenericChainLink struct {
	*ChainLink
}

func (b *GenericChainLink) GetSigId() SigId {
	return b.unpacked.sigId
}
func (b *GenericChainLink) Type() string            { return "generic" }
func (b *GenericChainLink) ToDisplayString() string { return "unknown" }
func (b *GenericChainLink) insertIntoTable(tab *IdentityTable) {
	tab.insertLink(b)
}
func (b *GenericChainLink) markRevoked(r TypedChainLink) {
	b.revoked = true
}
func (b *GenericChainLink) ToDebugString() string {
	return fmt.Sprintf("uid=%s, seq=%d, link=%s",
		string(b.parent.uid), b.unpacked.seqno, b.id.ToString())
}

func (g *GenericChainLink) IsRevocationIsh() bool { return false }
func (g *GenericChainLink) IsRevoked() bool       { return g.revoked }
func (g *GenericChainLink) IsActiveKey() bool     { return g.activeKey }
func (g *GenericChainLink) GetSeqno() Seqno       { return g.unpacked.seqno }
func (g *GenericChainLink) GetPgpFingerprint() PgpFingerprint {
	return g.unpacked.pgpFingerprint
}

func (g *GenericChainLink) GetCTime() time.Time {
	return time.Unix(int64(g.unpacked.ctime), 0)
}
func (g *GenericChainLink) GetArmoredSig() string {
	return g.unpacked.sig
}
func (g *GenericChainLink) GetUsername() string {
	return g.unpacked.username
}

//
//=========================================================================

//=========================================================================
// Remote, Web and Social
//
type RemoteProofChainLink interface {
	TypedChainLink
	TableKey() string
	LastWriterWins() bool
	GetRemoteUsername() string
	GetHostname() string
	GetProtocol() string
	DisplayCheck(*SigHint, *CheckResult, ProofError)
}

type WebProofChainLink struct {
	GenericChainLink
	protocol string
	hostname string
}

type SocialProofChainLink struct {
	GenericChainLink
	service  string
	username string
}

func (w *WebProofChainLink) TableKey() string {
	if w.protocol == "https" {
		return "http"
	} else {
		return w.protocol
	}
}

func (s *WebProofChainLink) DisplayCheck(
	hint *SigHint, cached *CheckResult, err ProofError) {
	var msg string
	if err == nil {
		if s.protocol == "dns" {
			msg = (CHECK + " admin of DNS zone " +
				ColorString("green", s.hostname) +
				": found TXT entry " + hint.checkText)
		} else {
			var color string
			if s.protocol == "https" {
				color = "green"
			} else {
				color = "yellow"
			}
			msg = (CHECK + " admin of " +
				ColorString(color, s.hostname) + " via " +
				ColorString(color, strings.ToUpper(s.protocol)) +
				": " + hint.humanUrl)
		}
	} else {
		msg = (BADX + " " +
			ColorString("red", "Proof for "+s.ToDisplayString()+" "+
				ColorString("bold", "failed")+": "+
				err.Error()))
	}
	if cached != nil {
		msg += " " + ColorString("magenta", cached.ToDisplayString())
	}
	G.OutputString(msg + "\n")
}

func (w *WebProofChainLink) Type() string { return "proof" }
func (w *WebProofChainLink) insertIntoTable(tab *IdentityTable) {
	remoteProofInsertIntoTable(w, tab)
}
func (w *WebProofChainLink) ToDisplayString() string {
	return w.protocol + "://" + w.hostname
}
func (w *WebProofChainLink) LastWriterWins() bool      { return false }
func (w *WebProofChainLink) GetRemoteUsername() string { return "" }
func (w *WebProofChainLink) GetHostname() string       { return w.hostname }
func (w *WebProofChainLink) GetProtocol() string       { return w.protocol }

func (s *SocialProofChainLink) TableKey() string { return s.service }
func (s *SocialProofChainLink) Type() string     { return "proof" }
func (s *SocialProofChainLink) insertIntoTable(tab *IdentityTable) {
	remoteProofInsertIntoTable(s, tab)
}
func (w *SocialProofChainLink) ToDisplayString() string {
	return w.username + "@" + w.service
}
func (s *SocialProofChainLink) LastWriterWins() bool      { return true }
func (s *SocialProofChainLink) GetRemoteUsername() string { return s.username }
func (w *SocialProofChainLink) GetHostname() string       { return "" }
func (w *SocialProofChainLink) GetProtocol() string       { return "" }

func NewWebProofChainLink(b GenericChainLink, p, h string) *WebProofChainLink {
	return &WebProofChainLink{b, p, h}
}
func NewSocialProofChainLink(b GenericChainLink, s, u string) *SocialProofChainLink {
	return &SocialProofChainLink{b, s, u}
}

func (s *SocialProofChainLink) DisplayCheck(
	hint *SigHint, cached *CheckResult, err ProofError) {

	var msg string
	if err == nil {
		msg = (CHECK + ` "` +
			ColorString("green", s.username) + `" on ` + s.service +
			": " + hint.humanUrl)
	} else {
		msg = (BADX +
			ColorString("red", ` "`+s.username+`" on `+s.service+" "+
				ColorString("bold", "failed")+": "+
				err.Error()))
	}
	if cached != nil {
		msg += " " + ColorString("magenta", cached.ToDisplayString())
	}
	G.OutputString(msg + "\n")
}

func ParseWebServiceBinding(base GenericChainLink) (
	ret RemoteProofChainLink, e error) {

	jw := base.payloadJson.AtKey("body").AtKey("service")

	if jw.IsNil() {
		ret = &SelfSigChainLink{base}

	} else if prot, e1 := jw.AtKey("protocol").GetString(); e1 == nil {

		var hostname string

		jw.AtKey("hostname").GetStringVoid(&hostname, &e1)
		if e1 == nil {
			switch prot {
			case "http:":
				ret = NewWebProofChainLink(base, "http", hostname)
			case "https:":
				ret = NewWebProofChainLink(base, "https", hostname)
			}
		} else if domain, e2 := jw.AtKey("domain").GetString(); e2 == nil && prot == "dns" {
			ret = NewWebProofChainLink(base, "dns", domain)
		}

	} else {

		var service, username string
		var e2 error

		jw.AtKey("name").GetStringVoid(&service, &e2)
		jw.AtKey("username").GetStringVoid(&username, &e2)
		if e2 == nil {
			ret = NewSocialProofChainLink(base, service, username)
		}
	}

	if ret == nil {
		e = fmt.Errorf("Unrecognized Web proof: %s @%s", jw.MarshalToDebug(),
			base.ToDebugString())
	}

	return
}

func remoteProofInsertIntoTable(l RemoteProofChainLink, tab *IdentityTable) {
	tab.insertLink(l)
	if k := l.TableKey(); len(k) > 0 {
		v, found := tab.remoteProofs[k]
		if !found {
			v = make([]RemoteProofChainLink, 0, 1)
		}
		v = append(v, l)
		tab.remoteProofs[k] = v
	}
}

//
//=========================================================================

//=========================================================================
// TrackChainLink
//
type TrackChainLink struct {
	GenericChainLink
	whom    string
	untrack *UntrackChainLink
}

func ParseTrackChainLink(b GenericChainLink) (ret *TrackChainLink, err error) {
	var whom string
	whom, err = b.payloadJson.AtPath("body.track.basics.username").GetString()
	if err != nil {
		err = fmt.Errorf("Bad track statement @%s: %s", b.ToDebugString(), err.Error())
	} else {
		ret = &TrackChainLink{b, whom, nil}
	}
	return
}

func (t *TrackChainLink) Type() string { return "track" }

func (b *TrackChainLink) ToDisplayString() string {
	return b.whom
}

func (l *TrackChainLink) insertIntoTable(tab *IdentityTable) {
	tab.insertLink(l)
	list, found := tab.tracks[l.whom]
	if !found {
		list = make([]*TrackChainLink, 0, 1)
	}
	list = append(list, l)
	tab.tracks[l.whom] = list
}

func (l *TrackChainLink) IsRevoked() bool {
	return l.revoked || l.untrack != nil
}

//
//=========================================================================

//=========================================================================
// UntrackChainLink
//

type UntrackChainLink struct {
	GenericChainLink
	whom string
}

func ParseUntrackChainLink(b GenericChainLink) (ret *UntrackChainLink, err error) {
	var whom string
	whom, err = b.payloadJson.AtPath("body.untrack.basics.username").GetString()
	if err != nil {
		err = fmt.Errorf("Bad track statement @%s: %s", b.ToDebugString(), err.Error())
	} else {
		ret = &UntrackChainLink{b, whom}
	}
	return
}

func (u *UntrackChainLink) insertIntoTable(tab *IdentityTable) {
	tab.insertLink(u)
	if list, found := tab.tracks[u.whom]; !found {
		G.Log.Notice("Bad untrack of %s; no previous tracking statement found",
			u.whom)
	} else {
		for _, obj := range list {
			obj.untrack = u
		}
	}
}

func (b *UntrackChainLink) ToDisplayString() string {
	return b.whom
}

func (r *UntrackChainLink) Type() string { return "untrack" }

func (r *UntrackChainLink) IsRevocationIsh() bool { return true }

//
//=========================================================================

//=========================================================================
// CryptocurrencyChainLink

type CryptocurrencyChainLink struct {
	GenericChainLink
	pkhash  []byte
	address string
}

func ParseCryptocurrencyChainLink(b GenericChainLink) (
	cl *CryptocurrencyChainLink, err error) {

	jw := b.payloadJson.AtPath("body.cryptocurrency")
	var typ, addr string
	var pkhash []byte

	jw.AtKey("type").GetStringVoid(&typ, &err)
	jw.AtKey("address").GetStringVoid(&addr, &err)

	if err != nil {
		return
	}

	if typ != "bitcoin" {
		err = fmt.Errorf("Can only handle 'bitcoin' addresses for now; got %s", typ)
		return
	}

	_, pkhash, err = BtcAddrCheck(addr, nil)
	if err != nil {
		err = fmt.Errorf("At signature %s: %s", b.ToDebugString(), err.Error())
		return
	}
	cl = &CryptocurrencyChainLink{b, pkhash, addr}
	return
}

func (r *CryptocurrencyChainLink) Type() string { return "cryptocurrency" }

func (r *CryptocurrencyChainLink) ToDisplayString() string { return r.address }

func (l *CryptocurrencyChainLink) insertIntoTable(tab *IdentityTable) {
	tab.insertLink(l)
	tab.cryptocurrency = append(tab.cryptocurrency, l)
}

func (l CryptocurrencyChainLink) Display() {
	msg := (BTC + " bitcoin " + ColorString("green", l.address))
	G.OutputString(msg + "\n")
}

//
//=========================================================================

//=========================================================================
// RevokeChainLink

type RevokeChainLink struct {
	GenericChainLink
}

func (r *RevokeChainLink) Type() string { return "revoke" }

func (r *RevokeChainLink) ToDisplayString() string {
	v := r.GetRevocations()
	list := make([]string, len(v), len(v))
	for i, s := range v {
		list[i] = s.ToString(true)
	}
	return strings.Join(list, ",")
}

func (r *RevokeChainLink) IsRevocationIsh() bool { return true }

func (l *RevokeChainLink) insertIntoTable(tab *IdentityTable) {
	tab.insertLink(l)
}

//
//=========================================================================

//=========================================================================
// SelfSigChainLink

type SelfSigChainLink struct {
	GenericChainLink
}

func (r *SelfSigChainLink) Type() string { return "self" }

func (s *SelfSigChainLink) ToDisplayString() string { return s.unpacked.username }

func (l *SelfSigChainLink) insertIntoTable(tab *IdentityTable) {
	tab.insertLink(l)
}
func (w *SelfSigChainLink) TableKey() string          { return "keybase" }
func (w *SelfSigChainLink) LastWriterWins() bool      { return true }
func (w *SelfSigChainLink) GetRemoteUsername() string { return w.GetUsername() }
func (w *SelfSigChainLink) GetHostname() string       { return "" }
func (w *SelfSigChainLink) GetProtocol() string       { return "" }

func (s *SelfSigChainLink) DisplayCheck(
	hint *SigHint, cached *CheckResult, err ProofError) {
	return
}

//
//=========================================================================

type IdentityTable struct {
	sigChain       *SigChain
	revocations    map[SigId]bool
	links          map[SigId]TypedChainLink
	remoteProofs   map[string][]RemoteProofChainLink
	tracks         map[string][]*TrackChainLink
	order          []TypedChainLink
	sigHints       *SigHints
	activeProofs   []RemoteProofChainLink
	cryptocurrency []*CryptocurrencyChainLink
}

func (tab *IdentityTable) insertLink(l TypedChainLink) {
	tab.links[l.GetSigId()] = l
	tab.order = append(tab.order, l)
	for _, rev := range l.GetRevocations() {
		tab.revocations[*rev] = true
		if targ, found := tab.links[*rev]; !found {
			G.Log.Warning("Can't revoke signature %s @%s",
				rev.ToString(true), l.ToDebugString())
		} else {
			targ.markRevoked(l)
		}
	}
}

func NewTypedChainLink(cl *ChainLink) (ret TypedChainLink, w Warning) {

	base := GenericChainLink{cl}

	s, err := cl.payloadJson.AtKey("body").AtKey("type").GetString()
	if len(s) == 0 || err != nil {
		err = fmt.Errorf("No type in signature @%s", base.ToDebugString())
	} else {
		switch s {
		case "web_service_binding":
			ret, err = ParseWebServiceBinding(base)
		case "track":
			ret, err = ParseTrackChainLink(base)
		case "untrack":
			ret, err = ParseUntrackChainLink(base)
		case "cryptocurrency":
			ret, err = ParseCryptocurrencyChainLink(base)
		case "revoke":
			ret = &RevokeChainLink{base}
		default:
			err = fmt.Errorf("Unknown signature type %s @%s", s, base.ToDebugString())
		}
	}

	if err != nil {
		w = ErrorToWarning(err)
		ret = &base
	}

	// Basically we never fail, since worse comes to worse, we treat
	// unknown signatures as "generic" and can still display them
	return
}

func NewIdentityTable(sc *SigChain, h *SigHints) *IdentityTable {
	ret := &IdentityTable{
		sigChain:       sc,
		revocations:    make(map[SigId]bool),
		links:          make(map[SigId]TypedChainLink),
		remoteProofs:   make(map[string][]RemoteProofChainLink),
		tracks:         make(map[string][]*TrackChainLink),
		order:          make([]TypedChainLink, 0, sc.Len()),
		sigHints:       h,
		activeProofs:   make([]RemoteProofChainLink, 0, sc.Len()),
		cryptocurrency: make([]*CryptocurrencyChainLink, 0, 0),
	}
	ret.Populate()
	ret.CollectAndDedupeActiveProofs()
	return ret
}

func (idt *IdentityTable) Populate() {
	G.Log.Debug("+ Populate ID Table")
	for _, link := range idt.sigChain.chainLinks {
		tl, w := NewTypedChainLink(link)
		tl.insertIntoTable(idt)
		if w != nil {
			G.Log.Warning(w.Warning())
		}
	}
	G.Log.Debug("- Populate ID Table")
}

func (idt *IdentityTable) ActiveCryptocurrency() *CryptocurrencyChainLink {
	var ret *CryptocurrencyChainLink
	tab := idt.cryptocurrency
	if len(tab) > 0 {
		last := tab[len(tab)-1]
		if !last.IsRevoked() {
			ret = last
		}
	}
	return ret
}

func (idt *IdentityTable) CollectAndDedupeActiveProofs() {
	seen := make(map[string]bool)
	tab := idt.activeProofs
	for _, list := range idt.remoteProofs {
		for i := len(list) - 1; i >= 0; i-- {
			link := list[i]
			if link.IsRevoked() {
				continue
			}

			// We only want to use the last proof in the list
			// if we have several (like for dns://chriscoyne.com)
			id := link.ToDisplayString()
			_, found := seen[id]
			if !found {
				tab = append(tab, link)
				seen[id] = true
			}

			// Things like Twitter, Github, etc, are last-writer wins.
			// Things like dns/https can have multiples
			if link.LastWriterWins() {
				break
			}
		}
	}
	idt.activeProofs = tab
}

func (idt *IdentityTable) Len() int {
	return len(idt.order)
}

func (idt *IdentityTable) Identify() error {
	var err error
	for _, activeProof := range idt.activeProofs {
		tmp := idt.IdentifyActiveProof(activeProof)
		if tmp != nil && err == nil {
			err = tmp
		}
	}
	acc := idt.ActiveCryptocurrency()
	if acc != nil {
		acc.Display()
	}

	if err != nil {
		err = fmt.Errorf("One or more proofs failed")
	}
	return err
}

//=========================================================================

func (idt *IdentityTable) IdentifyActiveProof(p RemoteProofChainLink) error {
	hint, cached, err := idt.CheckActiveProof(p)
	p.DisplayCheck(hint, cached, err)
	return err
}

func (idt *IdentityTable) CheckActiveProof(p RemoteProofChainLink) (
	hint *SigHint, cached *CheckResult, err ProofError) {

	sid := p.GetSigId()

	id := p.GetSigId()
	hint = idt.sigHints.Lookup(id)
	if hint == nil {
		err = NewProofError(PROOF_NO_HINT,
			"No server-given hint for sig=%s", id.ToString(true))
		return
	}

	if G.ProofCache != nil {
		if cached = G.ProofCache.Get(sid); cached != nil {
			err = cached.Status
			return
		}
	}

	var pc ProofChecker
	pc, err = NewProofChecker(p)

	if err != nil {
		return
	}

	err = pc.CheckHint(*hint)
	if err == nil {
		err = pc.CheckStatus(*hint)
	}

	p.MarkChecked(err)
	if G.ProofCache != nil {
		G.ProofCache.Put(sid, err)
	}

	return
}

//=========================================================================
