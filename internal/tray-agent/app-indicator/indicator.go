package app_indicator

import (
	"fmt"
	"github.com/liqotech/liqo-agent/internal/tray-agent/agent/client"
	"github.com/liqotech/liqo-agent/internal/tray-agent/icon"
	"github.com/liqotech/liqo-agent/internal/tray-agent/metrics"
	"sync"
	"time"
)

//standard width of an item in the tray menu
const menuWidth = 64

//Icon represents the icon displayed in the tray bar
type Icon int

//Icon displayed in the tray bar. It is internally mapped into one of the icons in
//github.com/liqotech/liqo-agent/assets/tray-agent/icons/tray-abr
const (
	IconLiqoMain Icon = iota
	IconLiqoNoConn
	IconLiqoOff
	IconLiqoWarning
	IconLiqoOrange
	IconLiqoGreen
	IconLiqoPurple
	IconLiqoRed
	IconLiqoYellow
	IconLiqoCyan
	IconLiqoNil
)

//graphicResource defines a graphic interaction handled by the Indicator.
type graphicResource int

//graphicResource currently handled by the Indicator.
const (
	resourceIcon graphicResource = iota
	resourceLabel
	resourceDesktop
)

//Run starts the Indicator execution, running the onReady() function. After Quit() call, it runs onExit() before
//exiting. It should be called at the very beginning of main() to lock at main thread.
func Run(onReady func(), onExit func()) {
	GetGuiProvider().Run(onReady, onExit)
}

//Indicator singleton
var root *Indicator

//Indicator is a stateful data structure that controls the app indicator and its related menu. It can be obtained
//and initialized calling GetIndicator()
type Indicator struct {
	//root node of the menu hierarchy.
	menu *MenuNode
	//indicator label showed in the tray bar along the tray icon
	label string
	//indicator icon-id
	icon Icon
	//TITLE MenuNode used by the indicator to show the menu header
	menuTitleNode *MenuNode
	//title text currently in use
	menuTitleText string
	//STATUS MenuNode used to display status information.
	menuStatusNode *MenuNode
	//map that stores QUICK MenuNodes, associating them with their tag
	quickMap map[string]*MenuNode
	//reference to the node of the ACTION currently selected. If none, it defaults to the ROOT node
	activeNode *MenuNode
	//data struct containing indicator config
	config *config
	//guiProvider to interact with the graphic server
	gProvider GuiProviderInterface
	//data struct containing Liqo Status, used to control the menuStatusNode
	status StatusInterface
	//controller of all the application goroutines
	quitChan chan struct{}
	//if true, quitChan is closed and Indicator can gracefully exit
	quitClosed bool
	//data struct that controls Agent interaction with the cluster
	agentCtrl *client.AgentController
	//map of all the instantiated Listeners
	listeners map[client.NotifyChannel]*Listener
	//map of all the instantiated Timers
	timers map[string]*Timer
	//graphicResource is the map containing the mutex to protect access to the graphic resources handled by the Indicator
	//(e.g. tray icon, tray label and desktop notifications).
	graphicResource map[graphicResource]*sync.RWMutex
}

//GetIndicator initializes and returns the Indicator singleton. This function should not be called before Run().
func GetIndicator() *Indicator {
	if root == nil {
		metrics.MTGetIndicator = metrics.NewMetricTimer("GetIndicator")
		root = &Indicator{
			quickMap:        make(map[string]*MenuNode),
			quitChan:        make(chan struct{}),
			listeners:       make(map[client.NotifyChannel]*Listener),
			timers:          make(map[string]*Timer),
			graphicResource: make(map[graphicResource]*sync.RWMutex),
		}
		root.graphicResource[resourceIcon] = &sync.RWMutex{}
		root.graphicResource[resourceLabel] = &sync.RWMutex{}
		root.graphicResource[resourceDesktop] = &sync.RWMutex{}
		root.gProvider = GetGuiProvider()
		root.SetIcon(IconLiqoNoConn)
		root.SetLabel("")
		root.menuTitleNode = newMenuNode(NodeTypeTitle, false, nil)
		root.menu = newMenuNode(NodeTypeRoot, false, nil)
		root.activeNode = root.menu
		root.menuStatusNode = newMenuNode(NodeTypeStatus, false, nil)
		root.config = newConfig()
		root.status = GetStatus()
		root.RefreshStatus()
		client.LoadLocalConfig()
		metrics.StopMetricTimer(metrics.MTGetIndicator, time.Now())
		root.agentCtrl = client.GetAgentController()
		if !root.agentCtrl.Connected() {
			root.ShowErrorNoConnection()
		} else if !root.agentCtrl.ValidConfiguration() {
			root.ShowError("LIQO AGENT - FATAL", "Agent could not retrieve configuration data.")
		} else {
			root.SetIcon(IconLiqoMain)
		}
	}
	return root
}

//-----ACTIONS-----

//AddAction adds an ACTION to the indicator menu. It is visible by default.
//
//	title : label displayed in the menu
//
//	tag : unique tag for the ACTION
//
//	callback : callback function to be executed at each 'clicked' event. If callback == nil, the function can be set
//	afterwards using (*Indicator).Connect() .
func (i *Indicator) AddAction(title string, tag string, callback func(args ...interface{}), args ...interface{}) *MenuNode {
	a := newMenuNode(NodeTypeAction, false, nil)
	a.parent = i.menu
	a.SetTitle(title)
	a.SetTag(tag)
	if callback != nil {
		a.Connect(false, callback, args...)
	}
	a.SetIsVisible(true)
	i.menu.actionMap[tag] = a
	return a
}

//Action returns the *MenuNode of the ACTION with this specific tag. If not present, present = false
func (i *Indicator) Action(tag string) (act *MenuNode, present bool) {
	act, present = i.menu.actionMap[tag]
	return
}

//AddQuick adds a QUICK to the indicator menu. It is visible by default.
//
//	title : label displayed in the menu
//
//	tag : unique tag for the QUICK
//
//	callback : callback function to be executed at each 'clicked' event. If callback == nil, the function can be set
//	afterwards using (*MenuNode).Connect() .
func (i *Indicator) AddQuick(title string, tag string, callback func(args ...interface{}), args ...interface{}) *MenuNode {
	q := newMenuNode(NodeTypeQuick, false, nil)
	q.parent = q
	q.SetTitle(title)
	q.SetTag(tag)
	if callback != nil {
		q.Connect(false, callback, args...)
	}
	q.SetIsVisible(true)
	i.quickMap[tag] = q
	return q
}

//-----QUICKS-----

//Quick returns the *MenuNode of the QUICK with this specific tag. If such QUICK does not exist, present == false.
func (i *Indicator) Quick(tag string) (quick *MenuNode, present bool) {
	quick, present = i.quickMap[tag]
	return
}

//-----GRAPHIC METHODS-----

//AddSeparator adds a separator line to the indicator menu
func (i *Indicator) AddSeparator() {
	i.gProvider.AddSeparator()
}

//SetMenuTitle sets the text content of the TITLE MenuNode, displayed as the menu header.
func (i *Indicator) SetMenuTitle(title string) {
	i.menuTitleNode.SetTitle(title)
	i.menuTitleNode.SetIsVisible(true)
	i.menuTitleText = title
}

//Icon returns the icon-id of the Indicator tray icon currently set.
func (i *Indicator) Icon() Icon {
	gr := i.graphicResource[resourceIcon]
	gr.RLock()
	defer gr.RUnlock()
	return i.icon
}

//SetIcon sets the Indicator tray icon. If 'ico' is not a valid argument or ico == IconLiqoNil,
//SetIcon does nothing.
func (i *Indicator) SetIcon(ico Icon) {
	gr := i.graphicResource[resourceIcon]
	var newIcon []byte
	switch ico {
	case IconLiqoNil:
		return
	case IconLiqoMain:
		newIcon = icon.LiqoMain
	case IconLiqoOff:
		newIcon = icon.LiqoOff
	case IconLiqoNoConn:
		newIcon = icon.LiqoNoConn
	case IconLiqoWarning:
		newIcon = icon.LiqoWarning
	case IconLiqoOrange:
		newIcon = icon.LiqoOrange
	case IconLiqoGreen:
		newIcon = icon.LiqoGreen
	case IconLiqoPurple:
		newIcon = icon.LiqoPurple
	case IconLiqoRed:
		newIcon = icon.LiqoRed
	case IconLiqoYellow:
		newIcon = icon.LiqoYellow
	case IconLiqoCyan:
		newIcon = icon.LiqoCyan
	default:
		return
	}
	gr.Lock()
	defer gr.Unlock()
	i.gProvider.SetIcon(newIcon)
	i.icon = ico
}

//Label returns the text content of Indicator tray label.
func (i *Indicator) Label() string {
	gr := i.graphicResource[resourceLabel]
	gr.RLock()
	defer gr.RUnlock()
	return i.label
}

//SetLabel sets the text content of Indicator tray label.
func (i *Indicator) SetLabel(label string) {
	gr := i.graphicResource[resourceLabel]
	gr.Lock()
	defer gr.Unlock()
	i.label = label
	i.gProvider.SetTitle(label)
}

//RefreshLabel updates the content of the Indicator label
//with the total number of both incoming and outgoing peerings currently active.
func (i *Indicator) RefreshLabel() {
	st := i.Status()
	in := st.Peerings(PeeringIncoming)
	out := st.Peerings(PeeringOutgoing)
	//since the label is graphically invasive, its content is displayed only when
	//there is at least one active peering
	if st.Running() && (in > 0 || out > 0) {
		i.SetLabel(fmt.Sprintf("(IN:%d/OUT:%d)", in, out))
		return
	}
	i.SetLabel("")
}

//--------------

//Quit stops the indicator execution.
func (i *Indicator) Quit() {
	metrics.DumpMetricResources()
	metrics.DumpMetricTimers()
	if i != nil {
		i.Disconnect()
		if i.agentCtrl.Connected() {
			i.agentCtrl.StopCaches()
		}
	}
	i.gProvider.Quit()
}

//Disconnect exits all the event handlers associated with any Indicator MenuNode via the Connect() method.
func (i *Indicator) Disconnect() {
	if !i.quitClosed {
		close(i.quitChan)
		i.quitClosed = true
	}
}

//AgentCtrl returns the Indicator AgentController that interacts with the cluster.
func (i *Indicator) AgentCtrl() *client.AgentController {
	return i.agentCtrl
}
