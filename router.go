package main

import (
	"math/rand"
	"time"
)

var (
	// Router will update every updateInterval + (randint() % updateDif)
	updateInterval = 2
	updateDiff     = 2
)

const (
	RIP_PORT    int    = 520
	BROADCAST   uint32 = 0xFFFFFFFF
	SINGLE_MASK uint32 = 0xFFFFFFFF
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type conn_t chan Packet

type Packet_rec_f func(Packet)

type tableEntry struct {
	ip     uint32
	mask   uint32
	metric uint32

	wayAddr uint32
}

type Router struct {
	name string
	ip   uint32
	mask uint32

	conn       conn_t               // Incomming packets are read from here
	connectios map[uint32]conn_t    // Direct connections to routers.
	lastHeard  map[uint32]time.Time // Time when last regular update was received. If it was too long ago, router is considered inactive
	table      []*tableEntry        // Routtin table

	accepting bool
	running   bool

	getListener      Packet_rec_f
	transmitListener Packet_rec_f

	timer int
}

func NewRouter(name string, ip, mask uint32) (r *Router) {
	r = new(Router)
	r.name = name
	r.ip = ip
	r.mask = mask
	r.connectios = make(map[uint32]conn_t)
	r.lastHeard = make(map[uint32]time.Time)
	r.table = make([]*tableEntry, 0)
	r.conn = make(conn_t, 256)
	r.running = false
	r.accepting = false

	r.getListener = nil
	r.transmitListener = nil

	r.ResetTimer()
	return
}

func (r *Router) ResetTimer() {
	r.timer = updateInterval + rand.Intn(updateDiff)
}

func (r *Router) handleBroadcast(p *Packet) {
	p.hopCount -= 1
	if p.hopCount > 0 {
		r.broadcast(p)
	}
}

func (r *Router) handlePacket(p *Packet) {
	if p.Port == RIP_PORT {
		r.handleRip(p.Data.(RipData), p.Sender)
	} else {
		if (p.Dest == BROADCAST) || (p.Dest == r.ip) {
			// This packet was sent to this router (or broadcast)
			if r.getListener != nil {
				r.getListener(*p)
			}
		} else {
			r.simpleSend(p)
			if r.transmitListener != nil {
				r.transmitListener(*p)
			}
		}
	}
}

func (r *Router) handleRip(rip RipData, sender uint32) {
	if rip.command == REQUEST {
		if (len(rip.entries) == 1) && (rip.entries[0].addressFamily == 0) && (rip.entries[0].metric > 15) {
			r.sendWholeTable(sender)
		} else {
			r.sendPart(sender, rip.entries)
		}
	} else if rip.command == RESPONSE {
		for _, e := range rip.entries {
			r.addEntry(e.ipv4Address, e.subnetMast, e.metric+1, sender)
		}
		//r.timer = 1
	}
	_, ok := r.lastHeard[sender]
	if ok {
		r.lastHeard[sender] = time.Now()
	}
	// In case the router was dead and has come back up, need to change that infinity metric
	r.addEntry(sender, SINGLE_MASK, 1, sender)
}

func (r *Router) routerLoop() {
	for packet := range r.conn {
		if r.accepting {
			r.handlePacket(&packet)
			if packet.Dest == BROADCAST {
				r.handleBroadcast(&packet)
			}
		}
	}
}

func (r *Router) checkExpired() {
	for ip, t := range r.lastHeard {
		if time.Since(t) > time.Duration(updateInterval*2+updateDiff)*time.Second {
			for i, e := range r.table {
				if e.wayAddr == ip {
					r.table[i].metric = 16
				}
			}
		}
	}
}

func (r *Router) regularUpdates() {
	for {
		time.Sleep(time.Second)
		if r.accepting {
			if r.timer <= 0 {
				r.ResetTimer()
				r.checkExpired()
				r.sendWholeTable(BROADCAST)
			} else {
				r.timer -= 1
			}
		}
	}
}

func min(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

func (r *Router) addEntry(ip, mask, metric, way uint32) {
	if r.ip == ip {
		return
	}
	metric = min(metric, 16)
	for _, entry := range r.table {
		if entry.ip == ip {
			if entry.metric > metric {
				entry.metric = metric
				entry.wayAddr = way
			} else {
				if entry.wayAddr == way {
					entry.metric = metric
				}
			}
			return
		}
	}
	e := new(tableEntry) //{ip, mask, metric, way}
	e.ip = ip
	e.mask = mask
	e.metric = metric
	e.wayAddr = way
	r.table = append(r.table, e)
}

func (r *Router) sendWholeTable(rcpt uint32) {
	if len(r.table) == 0 {
		return
	}
	var data RipData

	var sentOut bool = false
	for i, entry := range r.table {
		if i%25 == 0 {
			if i != 0 {
				// send out the old one
				p := NewPacket(data, rcpt, RIP_PORT)
				p.hopCount = 1
				r.Send(p)
				sentOut = true
			}
			// Build new packet
			data = NewRipData(RESPONSE)
		}
		data.AddEntry(NewRipEntry(entry.ip, entry.mask, entry.metric))
		sentOut = false
	}
	if !sentOut {
		p := NewPacket(data, rcpt, RIP_PORT)
		p.hopCount = 1
		r.Send(p)
	}
}

func (r *Router) requestWholeTable(rcpt uint32) {
	data := NewRipData(REQUEST)
	data.AddEntry(NewRipEntry(0x00, 0x00, 16))
	if rcpt == BROADCAST {
		for ip, _ := range r.connectios {
			p := NewPacket(data, ip, RIP_PORT)
			r.Send(p)
		}
	} else {
		p := NewPacket(data, rcpt, RIP_PORT)
		r.Send(p)
	}
}

func (r *Router) sendPart(rcpt uint32, entries []RipEntry) {
	// Not supported
	return
}

func (r *Router) addDirectConn(ip uint32, c conn_t) {
	r.connectios[ip] = c
	r.lastHeard[ip] = time.Now()
	r.addEntry(ip, SINGLE_MASK, 1, ip)
}

func (r *Router) findWay(ip uint32) (bool, uint32) {
	for _, entry := range r.table {
		if entry.ip == ip {
			if entry.metric > 15 {
				return false, 0x00
			}
			return true, entry.wayAddr
		}
	}

	return false, 0x00
}

func (r *Router) broadcast(p *Packet) {
	oldSender := p.Sender
	p.Sender = r.ip
	for ip, conn := range r.connectios {
		if ip != oldSender {
			conn <- *p
		}
	}
}

func (r *Router) simpleSend(p *Packet) bool {
	f, way := r.findWay(p.Dest)
	if f {
		conn := r.connectios[way]
		conn <- *p
		return true
	} else {
		return false
	}
}

func (r *Router) Start() {
	r.accepting = true
	if !r.running {
		r.running = true
		go r.routerLoop()
	}
	r.requestWholeTable(BROADCAST)
	go r.regularUpdates()
}

func Connect(r1, r2 *Router) {
	r1.addDirectConn(r2.ip, r2.conn)
	r2.addDirectConn(r1.ip, r1.conn)
}

func (r *Router) Send(p Packet) bool {
	p.Sender = r.ip
	if p.Dest == BROADCAST {
		r.broadcast(&p)
		return true
	} else {
		return r.simpleSend(&p)
	}
}

func (r *Router) AddGetListener(f Packet_rec_f) {
	r.getListener = f
}

func (r *Router) AddTransmitListener(f Packet_rec_f) {
	r.transmitListener = f
}
