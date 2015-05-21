package main

const (
	REQUEST  uint8 = 1
	RESPONSE uint8 = 2
)

type Data_t interface {
}

type Packet struct {
	Port   int
	Dest   uint32
	Sender uint32

	hopCount uint32

	Data Data_t
}

func NewPacket(data Data_t, dest uint32, port int) (p Packet) {
	p.Port = port
	p.Dest = dest
	p.Data = data
	p.hopCount = 16
	return
}

type RipEntry struct {
	addressFamily uint16 // IGONER
	routeTag      uint16 // IGNORE
	ipv4Address   uint32
	subnetMast    uint32
	nextHop       uint32 // IGNORE
	metric        uint32
}

func NewRipEntry(ip, mask, metric uint32) (entry RipEntry) {
	entry.addressFamily = 0x0000
	entry.ipv4Address = ip
	entry.subnetMast = mask
	entry.metric = metric
	return
}

type RipData struct {
	command uint8
	version uint8
	zero    uint16
	entries []RipEntry
}

func NewRipData(command uint8) (packet RipData) {
	packet.entries = make([]RipEntry, 0)
	packet.zero = 0x0000
	packet.command = command
	return
}

func (packet *RipData) AddEntry(entry RipEntry) {
	if len(packet.entries) >= 25 {
		return
	}
	packet.entries = append(packet.entries, entry)
}

type Text struct {
	text string
}

func NewText(msg string) (t Text) {
	t.text = msg
	return
}
