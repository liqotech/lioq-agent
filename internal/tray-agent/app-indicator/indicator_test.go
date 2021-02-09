package app_indicator

import (
	"github.com/liqotech/liqo-agent/internal/tray-agent/agent/client"
	"github.com/stretchr/testify/assert"
	"testing"
)

// test Indicator startup configuration and basic methods
func TestGetIndicator(t *testing.T) {
	UseMockedGuiProvider()
	client.UseMockedAgentController()
	DestroyMockedIndicator()
	client.DestroyMockedAgentController()
	i := GetIndicator()
	i.AddSeparator()
	assert.NotNil(t, i.menu, "root MenuNode not instantiated")
	assert.NotNil(t, i.quickMap, "root quickMap not instantiated")
	assert.NotNil(t, i.Config(), "root config obj not instantiated")
	assert.NotNil(t, i.quitChan, "root quitChan not instantiated")
	assert.NotNil(t, i.listeners, "root listeners not instantiated")
	if assert.NotNil(t, i.AgentCtrl(), "root agentCtrl obj not instantiated") {
		if i.agentCtrl.Connected() {
			assert.Equal(t, IconLiqoMain, i.Icon())
		} else {
			assert.Equal(t, IconLiqoNoConn, i.Icon())
		}
	}
	i.SetLabel("test")
	assert.Equal(t, "test", i.Label())
	i.SetMenuTitle("test")
	assert.Equal(t, i.menuTitleText, "test", "Indicator menu title not correctly set")
	assert.True(t, i.menuTitleNode.isVisible, "Indicator menu title node not visible")
	i.Quit()
}

// simulation of an Indicator routine that allows to test functions of Indicator and MenuNode
func TestIndicatorRoutine(t *testing.T) {
	UseMockedGuiProvider()
	client.UseMockedAgentController()
	DestroyMockedIndicator()
	client.DestroyMockedAgentController()
	i := GetIndicator()
	// test QUICKs registration
	i.AddQuick("test", "QUICK_TAG_1", nil)
	i.AddQuick("click test", "QUICK_CLICK_TEST", nil)
	assert.Equal(t, 2, len(i.quickMap), "QUICKs not correctly registered")
	// test QUICK status: AddQuick
	for _, q := range i.quickMap {
		quick, present := i.Quick(q.Tag())
		assert.Truef(t, present, "QUICK %s not registered", q.tag)
		assert.NotNilf(t, quick, "QUICK node %s is nil", q.tag)
		assert.Truef(t, q.isVisible, "QUICK node %s is not visible", q.tag)
	}
	// test ACTION registration
	i.AddAction("test action", "ACTION_TAG", nil)
	assert.Equal(t, 1, len(i.menu.actionMap), "ACTION not correctly registered")
	a, ok1 := i.Action("ACTION_TAG")
	assert.True(t, ok1, "ACTION not registered")
	assert.NotNil(t, a, "ACTION node is nil")
	assert.True(t, a.IsVisible(), "ACTION node is not visible")
	// test OPTION registration
	a.AddOption("option test", "OPTION_TAG", "", false, nil, nil)
	o, ok2 := a.Option("OPTION_TAG")
	assert.True(t, ok2, "OPTION not registered")
	assert.NotNil(t, a, "OPTION node is nil")
	// test MenuNode operations
	// test LIST MenuNode operations
	// - insertion
	child1 := a.UseListChild("", "child1")
	child2 := a.UseListChild("", "child2")
	assert.NotNil(t, child1, "child node 1 creation failed")
	assert.NotNil(t, child2, "child node 2 creation failed")
	// - get
	_, childPresent := a.ListChild("child1")
	assert.True(t, childPresent, "failed retrieval of inserted LIST child")
	assert.Equal(t, 0, a.nodeList.totFree, "there should be 0 free LIST children available")
	assert.Equal(t, 2, len(a.nodeList.usedNodes), "there should be 2 LIST children used")
	// - remove single child
	a.FreeListChild("child2")
	assert.Equal(t, 1, a.nodeList.totFree, "there should be 1 free LIST child available")
	assert.Equal(t, 1, len(a.nodeList.usedNodes), "there should be 1 LIST child used")
	// - remove all children
	a.FreeListChildren()
	assert.Equal(t, 2, a.nodeList.totFree, "there should be 1 free LIST child available")
	assert.Equal(t, 0, len(a.nodeList.usedNodes), "there should be 1 LIST child used")

	// test Disconnect()
	assert.False(t, a.stopped, "node stopChan is closed before Disconnect()")
	a.Disconnect()
	assert.True(t, a.stopped, "node stopChan is not closed after Disconnect()")
	// test getters/setters
	oItem := o.item.(*mockItem)
	assert.True(t, o.IsVisible(), "OPTION is hidden")
	assert.True(t, oItem.Visible(), "item of visible MenuNode is marked not visible")
	o.SetIsVisible(false)
	assert.False(t, o.isVisible, "OPTION is not visible")
	assert.False(t, oItem.Visible(), "item of hidden MenuNode is visible")
	//
	o.SetIsEnabled(false)
	assert.False(t, o.IsEnabled(), "OPTION is enabled")
	o.SetIsEnabled(true)
	assert.True(t, o.IsEnabled(), "OPTION is not enabled")
	//
	o.SetIsChecked(true)
	assert.True(t, o.IsChecked(), "OPTION is not checked")
	o.SetIsChecked(false)
	assert.False(t, o.IsChecked(), "OPTION is checked")
	assert.Equal(t, o.title, oItem.Title(), "title of unchecked MenuNode not correctly reverted to normal")

	o.SetIsInvalid(true)
	assert.True(t, o.IsInvalid(), "OPTION is not invalid")
	o.SetIsInvalid(false)
	assert.False(t, o.IsInvalid(), "OPTION is invalid")
	// test SelectAction()/DeselectAction() :only selected action should be visible
	o.SetIsVisible(false)
	//test Quit() and Disconnect
	i.Quit()
	assert.True(t, i.quitClosed, "Indicator quitChan not closed at Quit()")
}

func TestMenuNode_Connect(t *testing.T) {
	UseMockedGuiProvider()
	client.UseMockedAgentController()
	DestroyMockedIndicator()
	client.DestroyMockedAgentController()
	i := GetIndicator()
	et := GetGuiProvider().NewEventTester()
	et.Test()
	flagTest := false
	o := i.AddQuick("test flag", "test", func(args ...interface{}) {
		fp := args[0].(*bool)
		*fp = true
	}, &flagTest)
	ch := o.Channel()
	assert.NotNil(t, ch)
	et.Add(1)
	ch <- struct{}{}
	et.Wait()
	assert.True(t, flagTest, "Connect() callback not executed")
	i.Quit()
}
