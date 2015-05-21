package main

import (
	"fmt"
	"github.com/mattn/go-gtk/gdk"
	"github.com/mattn/go-gtk/glib"
	"github.com/mattn/go-gtk/gtk"
	"unsafe"
)

var (
	window      *gtk.Window      = nil
	drawingarea *gtk.DrawingArea = nil
	gdkwin      *gdk.Window      = nil
	pixmap      *gdk.Pixmap      = nil
	gc          *gdk.GC          = nil
	statusBar   *gtk.Statusbar   = nil
	configured  bool             = false
	font        *gdk.Font        = nil
)

type Mode uint8

const (
	SELECT Mode = iota
	INSERT
	CONNECT
	MESSAGE
)

var Selected *RouterView = nil
var CurrentMode Mode = SELECT

type Status uint

const (
	RECEIVED Status = iota
	TRANSFERED
	WORKING
)

const (
	PLOT_WIDTH    int = 800
	PLOT_HEIGTH   int = 500
	ROUTER_WIDTH  int = 20
	ROUTER_HEIGTH int = 20
	LINE_WIDTH    int = 6
)

var (
	COL_BG         = gdk.NewColorRGB(0xff, 0xff, 0xff) // Background color
	COL_FG         = gdk.NewColorRGB(0x37, 0x3b, 0x41) // Text colort
	COL_CONN       = COL_FG                            // Connection color
	COL_IDLE       = gdk.NewColorRGB(0x19, 0x88, 0x44) // Idle router color
	COL_STOPED     = gdk.NewColorRGB(0xcc, 0x34, 0x2b) // Router that's ignoring packets
	COL_TRANSFERED = gdk.NewColorRGB(0x39, 0x71, 0xed) // Router that was part of a routing path
	COL_RECEIVED   = COL_TRANSFERED                    // Router that received packet for it or got a broadcast
	COL_SELECED    = gdk.NewColorRGB(0xfb, 0xa9, 0x22)

	connections []Connection  = make([]Connection, 0)
	routers     []*RouterView = make([]*RouterView, 0)
)

func ipFormat(ip uint32) string {
	n1 := uint8((ip >> (8 * 3)) & 0xFF)
	n2 := uint8((ip >> (8 * 2)) & 0xFF)
	n3 := uint8((ip >> (8 * 1)) & 0xFF)
	n4 := uint8(ip & 0xFF)

	return fmt.Sprintf("%03d.%03d.%03d.%03d", n1, n2, n3, n4)
}

type RouterView struct {
	router *Router
	x      int
	y      int
	status Status
}

func NewRouterView(ip uint32, x, y int) *RouterView {
	v := new(RouterView)
	v.router = NewRouter(ipFormat(ip), ip, SINGLE_MASK)
	v.x = x
	v.y = y
	v.status = WORKING
	v.router.AddGetListener(func(p Packet) {
		v.status = RECEIVED
		Redraw()
	})
	v.router.AddTransmitListener(func(p Packet) {
		v.status = TRANSFERED
		Redraw()
	})
	return v
}

func (r *RouterView) calcColor() *gdk.Color {
	if r.status == RECEIVED {
		return COL_RECEIVED
	}
	if r.status == TRANSFERED {
		return COL_TRANSFERED
	}
	if r.router.accepting {
		return COL_IDLE
	} else {
		return COL_STOPED
	}
}

func (r *RouterView) orgin() (int, int) {
	return r.x - (ROUTER_WIDTH / 2), r.y - (ROUTER_HEIGTH / 2)
}

func (r *RouterView) Draw() {
	ox, oy := r.orgin()
	if r == Selected {
		sw := 2
		Rectangle(ox-sw, oy-sw, ROUTER_WIDTH+(2*sw), ROUTER_WIDTH+(2*sw), COL_SELECED)
	}
	col := r.calcColor()
	Rectangle(ox, oy, ROUTER_WIDTH, ROUTER_HEIGTH, col)

	SetFg(COL_FG)
	Write(ox-20, oy-5, ipFormat(r.router.ip))
}

func (r *RouterView) Toggle() {
	if !r.router.running {
		r.router.Start()
	} else {
		r.router.accepting = !r.router.accepting
	}

	Redraw()
}

type Connection struct {
	r1, r2 *RouterView
}

func (c *Connection) Contains(r *RouterView) bool {
	return (c.r1 == r) || (c.r2 == r)
}

func (c *Connection) Draw() {
	Line(c.r1.x, c.r1.y, c.r2.x, c.r2.y, COL_CONN)
}

func ConnectionExists(r1, r2 *RouterView) bool {
	for _, c := range connections {
		if c.Contains(r1) && c.Contains(r2) {
			return true
		}
	}
	return false
}

func AddConnection(r1, r2 *RouterView) {
	if ConnectionExists(r1, r2) {
		return
	}
	Connect(r1.router, r2.router)
	connections = append(connections, Connection{r1, r2})
}

func AddRouter(r *RouterView) {
	for _, rt := range routers {
		if rt.router == r.router {
			return
		}
	}
	routers = append(routers, r)
}

func ResetStatus() {
	for _, rw := range routers {
		rw.status = WORKING
	}
}

func RouterSelected(x, y int) *RouterView {
	for _, r := range routers {
		ox, oy := r.orgin()
		if !((x < ox) || (x > ox+ROUTER_WIDTH) || (y < oy) || (y > oy+ROUTER_HEIGTH)) {
			return r
		}
	}
	return nil
}

var nextIP uint32 = 0x00000001

func insertNew(x, y int) {
	r := NewRouterView(nextIP, x, y)
	nextIP += 1
	AddRouter(r)
	Selected = r
	Redraw()
}

func tryConnect(rw *RouterView) {
	if (Selected != rw) && (Selected != nil) {
		AddConnection(Selected, rw)
		Redraw()
	}
}

func sendMessage(rw *RouterView) {
	if (Selected != rw) && (Selected != nil) {
		p := NewPacket("data", rw.router.ip, 80)
		if Selected.router.Send(p) {
			Selected.status = TRANSFERED
		}
	}
}

func printTable(rw *RouterView) {
	if rw == nil {
		return
	}
	r := rw.router
	fmt.Println(ipFormat(r.ip), ":")
	for _, e := range r.table {
		fmt.Printf("  %s  [%2d]  %s\n", ipFormat(e.ip), e.metric, ipFormat(e.wayAddr))
	}
}

func Redraw() {
	if !configured {
		return
	}
	Clear()
	for _, c := range connections {
		c.Draw()
	}
	for _, r := range routers {
		r.Draw()
	}
	drawingarea.GetWindow().Invalidate(nil, false)

}

func SetFg(c *gdk.Color) {
	gc.SetRgbFgColor(c)
}

func Rectangle(x, y, w, h int, col *gdk.Color) {
	SetFg(col)
	pixmap.GetDrawable().DrawRectangle(gc, true, x, y, w, h)
}

func Line(x, y, x1, y1 int, col *gdk.Color) {
	SetFg(col)
	pixmap.GetDrawable().DrawLine(gc, x, y, x1, y1)
}

func Write(x, y int, s string) {
	if font == nil {
		font = gdk.FontLoad("-*-clean-medium-r-normal-*-12-*-*-*-*-*-*-*")
	}
	pixmap.GetDrawable().DrawString(font, gc, x, y, s)
}

func Clear() {
	if configured {
		Rectangle(0, 0, -1, -1, COL_BG)
	}
}

func selectedChanged() {
	if Selected != nil {
		statusBar.Push(0, ipFormat(Selected.router.ip))
	} else {
		statusBar.Push(0, "")
	}
}

func HandleButton(x, y int) {
	rw := RouterSelected(x, y)
	ResetStatus()

	switch CurrentMode {
	case SELECT:
		Selected = rw
		selectedChanged()
		break
	case INSERT:
		if rw == nil {
			insertNew(x, y)
			selectedChanged()
		}
		break
	case CONNECT:
		if rw != nil {
			tryConnect(rw)
		}
		break
	case MESSAGE:
		if rw != nil {
			sendMessage(rw)
		}
		break
	}
	CurrentMode = SELECT
	Redraw()
}

func insertMode() {
	CurrentMode = INSERT
}

func selectMode() {
	CurrentMode = SELECT
}

func toggleSelected() {
	rw := Selected
	if rw != nil {
		rw.Toggle()
	}
}

func connectMode() {
	CurrentMode = CONNECT
}

func messageMode() {
	CurrentMode = MESSAGE
}

func printSelected() {
	printTable(Selected)
}

func HandleKey(k uint) {
	switch k {
	case gdk.KEY_i:
		insertMode()
		return
	case gdk.KEY_s:
		selectMode()
		return
	case gdk.KEY_t:
		toggleSelected()
		return
	case gdk.KEY_c:
		connectMode()
		return
	case gdk.KEY_m:
		messageMode()
		return
	case gdk.KEY_p:
		printSelected()
		return
	case gdk.KEY_T:
		for _, rw := range routers {
			r := rw.router
			if !r.running {
				r.Start()
			} else {
				r.accepting = true
			}
		}
		Redraw()
		return
	}
}

func fillButtons(hbox *gtk.HBox) {
	bInsert := gtk.NewButtonWithLabel("Insert (i)")
	bInsert.Connect("clicked", insertMode)

	bSelect := gtk.NewButtonWithLabel("Select (s)")
	bSelect.Connect("clicked", selectMode)

	bConnect := gtk.NewButtonWithLabel("Connect (c)")
	bConnect.Connect("clicked", connectMode)

	bToggle := gtk.NewButtonWithLabel("Toggle (t)")
	bToggle.Connect("clicked", toggleSelected)

	bMessage := gtk.NewButtonWithLabel("Send (m)")
	bMessage.Connect("clicked", messageMode)

	bPrint := gtk.NewButtonWithLabel("Print (p)")
	bPrint.Connect("clicked", printSelected)

	hbox.Add(bInsert)
	hbox.Add(bSelect)
	hbox.Add(bConnect)
	hbox.Add(bToggle)
	hbox.Add(bMessage)
	hbox.Add(bPrint)
}

func main() {
	gtk.Init(nil)
	window = gtk.NewWindow(gtk.WINDOW_TOPLEVEL)
	window.SetTitle("Netsim")
	window.Connect("destroy", gtk.MainQuit)

	vbox := gtk.NewVBox(false, 5)
	drawingarea = gtk.NewDrawingArea()
	drawingarea.SetSizeRequest(int(PLOT_WIDTH), int(PLOT_HEIGTH))

	drawingarea.Connect("configure-event", func() {
		if pixmap != nil {
			pixmap.Unref()
		}
		allocation := drawingarea.GetAllocation()
		pixmap = gdk.NewPixmap(drawingarea.GetWindow().GetDrawable(), allocation.Width, allocation.Height, 24)
		gc = gdk.NewGC(pixmap.GetDrawable())
		configured = true

		Clear()
	})

	drawingarea.Connect("button-press-event", func(ctx *glib.CallbackContext) {
		if gdkwin == nil {
			gdkwin = drawingarea.GetWindow()
		}
		arg := ctx.Args(0)
		mev := *(**gdk.EventButton)(unsafe.Pointer(&arg))
		x, y := int(mev.X), int(mev.Y)
		HandleButton(x, y)
	})

	drawingarea.Connect("expose-event", func() {
		if pixmap != nil {
			drawingarea.GetWindow().GetDrawable().DrawDrawable(gc, pixmap.GetDrawable(), 0, 0, 0, 0, -1, -1)

		}
	})

	window.Connect("key-press-event", func(ctx *glib.CallbackContext) {
		arg := ctx.Args(0)
		var ev *gdk.EventKey = *(**gdk.EventKey)(unsafe.Pointer(&arg))
		HandleKey(uint(ev.Keyval))
	})

	drawingarea.AddEvents(int(gdk.BUTTON_PRESS_MASK))

	hbox := gtk.NewHBox(false, 10)
	hbox.SetSizeRequest(PLOT_WIDTH, 30)

	statusBar = gtk.NewStatusbar()
	fillButtons(hbox)

	vbox.Add(drawingarea)

	vbox.Add(hbox)
	vbox.Add(statusBar)

	window.Add(vbox)
	window.SetResizable(false)
	window.ShowAll()

	gtk.Main()

}
