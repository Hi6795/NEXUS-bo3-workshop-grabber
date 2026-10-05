//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"
)

const (
	appTitle = "NEXUS BO3 Workshop Grabber v0.2"

	WM_DESTROY        = 0x0002
	WM_COMMAND        = 0x0111
	WM_SETFONT        = 0x0030
	WM_CTLCOLORBTN    = 0x0135
	WM_CTLCOLOREDIT   = 0x0133
	WM_CTLCOLORSTATIC = 0x0138

	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_VISIBLE          = 0x10000000
	WS_CHILD            = 0x40000000
	WS_TABSTOP          = 0x00010000
	WS_VSCROLL          = 0x00200000
	WS_BORDER           = 0x00800000
	WS_EX_CLIENTEDGE = 0x00000200

	ES_LEFT        = 0x0000
	ES_MULTILINE   = 0x0004
	ES_AUTOVSCROLL = 0x0040
	ES_WANTRETURN  = 0x1000
	ES_READONLY    = 0x0800
	ES_AUTOHSCROLL = 0x0080
	BS_PUSHBUTTON = 0x00000000
	SW_SHOW = 5
	COLOR_WINDOW = 5
	IDC_ARROW    = 32512
	TRANSPARENT = 1
	DWMWA_USE_IMMERSIVE_DARK_MODE = 20
	EM_SETSEL     = 0x00B1
	EM_REPLACESEL = 0x00C2
	MB_OK              = 0x00000000
	MB_ICONERROR       = 0x00000010
	MB_ICONINFORMATION = 0x00000040
	MB_ICONWARNING     = 0x00000030
	BIF_RETURNONLYFSDIRS = 0x00000001
	BIF_NEWDIALOGSTYLE   = 0x00000040
	COINIT_APARTMENTTHREADED = 0x2

	ID_INPUT     = 1001
	ID_OUTPUT    = 1002
	ID_BROWSE    = 1003
	ID_DOWNLOAD  = 1004
	ID_CANCEL    = 1005
	ID_OPEN      = 1006
	ID_TESTITEMS = 1007
	ID_CLEAR     = 1008
	ID_LOG       = 1009
)

type point struct{ X, Y int32 }
type msg struct {
	Hwnd uintptr
	Message uint32
	WParam uintptr
	LParam uintptr
	Time uint32
	Pt point
	LPrivate uint32
}
type wndClassEx struct {
	CbSize uint32
	Style uint32
	LpfnWndProc uintptr
	CbClsExtra int32
	CbWndExtra int32
	HInstance uintptr
	HIcon uintptr
	HCursor uintptr
	HbrBackground uintptr
	LpszMenuName *uint16
	LpszClassName *uint16
	HIconSm uintptr
}
type browseInfo struct {
	HwndOwner uintptr
	PidlRoot uintptr
	PszDisplayName *uint16
	LpszTitle *uint16
	UlFlags uint32
	Lpfn uintptr
	LParam uintptr
	IImage int32
}

var (
	user32=syscall.NewLazyDLL("user32.dll")
	kernel32=syscall.NewLazyDLL("kernel32.dll")
	gdi32=syscall.NewLazyDLL("gdi32.dll")
	uxtheme=syscall.NewLazyDLL("uxtheme.dll")
	dwmapi=syscall.NewLazyDLL("dwmapi.dll")
	shell32=syscall.NewLazyDLL("shell32.dll")
	ole32=syscall.NewLazyDLL("ole32.dll")

	pRegisterClassExW=user32.NewProc("RegisterClassExW")
	pCreateWindowExW=user32.NewProc("CreateWindowExW")
	pDefWindowProcW=user32.NewProc("DefWindowProcW")
	pShowWindow=user32.NewProc("ShowWindow")
	pUpdateWindow=user32.NewProc("UpdateWindow")
	pGetMessageW=user32.NewProc("GetMessageW")
	pTranslateMessage=user32.NewProc("TranslateMessage")
	pDispatchMessageW=user32.NewProc("DispatchMessageW")
	pPostQuitMessage=user32.NewProc("PostQuitMessage")
	pLoadCursorW=user32.NewProc("LoadCursorW")
	pSendMessageW=user32.NewProc("SendMessageW")
	pSetWindowTextW=user32.NewProc("SetWindowTextW")
	pGetWindowTextLengthW=user32.NewProc("GetWindowTextLengthW")
	pGetWindowTextW=user32.NewProc("GetWindowTextW")
	pEnableWindow=user32.NewProc("EnableWindow")
	pMessageBoxW=user32.NewProc("MessageBoxW")
	pSetProcessDPIAware=user32.NewProc("SetProcessDPIAware")

	pGetModuleHandleW=kernel32.NewProc("GetModuleHandleW")
	pCreateFontW=gdi32.NewProc("CreateFontW")
	pCreateSolidBrush=gdi32.NewProc("CreateSolidBrush")
	pSetTextColor=gdi32.NewProc("SetTextColor")
	pSetBkColor=gdi32.NewProc("SetBkColor")
	pSetBkMode=gdi32.NewProc("SetBkMode")
	pSetWindowTheme=uxtheme.NewProc("SetWindowTheme")
	pDwmSetWindowAttribute=dwmapi.NewProc("DwmSetWindowAttribute")
	pSHBrowseForFolderW=shell32.NewProc("SHBrowseForFolderW")
	pSHGetPathFromIDListW=shell32.NewProc("SHGetPathFromIDListW")
	pCoTaskMemFree=ole32.NewProc("CoTaskMemFree")
	pCoInitializeEx=ole32.NewProc("CoInitializeEx")
	pCoUninitialize=ole32.NewProc("CoUninitialize")

	hwndMain uintptr
	hwndInput uintptr
	hwndOutput uintptr
	hwndBrowse uintptr
	hwndDownload uintptr
	hwndCancel uintptr
	hwndOpen uintptr
	hwndTest uintptr
	hwndClear uintptr
	hwndLog uintptr
	hwndStatus uintptr
	normalFont uintptr
	titleFont uintptr
	darkBgBrush uintptr
	darkEditBrush uintptr
	currentCancel context.CancelFunc
	running atomic.Bool
)

func utf16Ptr(s string)*uint16{ p,_:=syscall.UTF16PtrFromString(s); return p }
func createWindow(exStyle uint32,class,text string,style uint32,x,y,w,h int32,parent,id uintptr)uintptr{
	ret,_,_:=pCreateWindowExW.Call(uintptr(exStyle),uintptr(unsafe.Pointer(utf16Ptr(class))),uintptr(unsafe.Pointer(utf16Ptr(text))),uintptr(style),uintptr(x),uintptr(y),uintptr(w),uintptr(h),parent,id,0,0)
	if parent!=0{applyDarkTheme(ret)}
	return ret
}
func setText(hwnd uintptr,s string){pSetWindowTextW.Call(hwnd,uintptr(unsafe.Pointer(utf16Ptr(s))))}
func getText(hwnd uintptr)string{
	n,_,_:=pGetWindowTextLengthW.Call(hwnd)
	buf:=make([]uint16,n+1)
	pGetWindowTextW.Call(hwnd,uintptr(unsafe.Pointer(&buf[0])),n+1)
	return syscall.UTF16ToString(buf)
}
func enable(hwnd uintptr,yes bool){v:=uintptr(0);if yes{v=1};pEnableWindow.Call(hwnd,v)}
func messageBox(title,text string,flags uintptr){pMessageBoxW.Call(hwndMain,uintptr(unsafe.Pointer(utf16Ptr(text))),uintptr(unsafe.Pointer(utf16Ptr(title))),flags)}
func sendFont(hwnd,font uintptr){pSendMessageW.Call(hwnd,WM_SETFONT,font,1)}
func rgb(r,g,b byte)uintptr{return uintptr(r)|uintptr(g)<<8|uintptr(b)<<16}
func applyDarkTheme(hwnd uintptr){if hwnd!=0{pSetWindowTheme.Call(hwnd,uintptr(unsafe.Pointer(utf16Ptr("DarkMode_Explorer"))),0)}}
func applyDarkTitleBar(hwnd uintptr){if hwnd==0{return};enabled:=int32(1);pDwmSetWindowAttribute.Call(hwnd,DWMWA_USE_IMMERSIVE_DARK_MODE,uintptr(unsafe.Pointer(&enabled)),unsafe.Sizeof(enabled))}
func appendLog(line string){if hwndLog==0{return};line=strings.TrimRight(line,"\r\n")+"\r\n";pSendMessageW.Call(hwndLog,EM_SETSEL,^uintptr(0),^uintptr(0));pSendMessageW.Call(hwndLog,EM_REPLACESEL,0,uintptr(unsafe.Pointer(utf16Ptr(line))))}
func clearLog(){setText(hwndLog,"")}
func setStatus(s string){setText(hwndStatus,s)}

func createFonts(){
	normalFont,_,_=pCreateFontW.Call(18,0,0,0,400,0,0,0,1,0,0,5,0,uintptr(unsafe.Pointer(utf16Ptr("Segoe UI"))))
	titleFont,_,_=pCreateFontW.Call(30,0,0,0,700,0,0,0,1,0,0,5,0,uintptr(unsafe.Pointer(utf16Ptr("Segoe UI"))))
}
func createUI(){
	createFonts()
	h:=createWindow(0,"STATIC","NEXUS BO3 Workshop Grabber",WS_CHILD|WS_VISIBLE,22,18,620,42,hwndMain,0);sendFont(h,titleFont)
	h=createWindow(0,"STATIC","Paste BO3 Workshop URLs or numeric IDs — one per line. BO3 does not need to be installed.",WS_CHILD|WS_VISIBLE,24,58,820,24,hwndMain,0);sendFont(h,normalFont)
	hwndInput=createWindow(WS_EX_CLIENTEDGE,"EDIT","",WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|ES_LEFT|ES_MULTILINE|ES_AUTOVSCROLL|ES_WANTRETURN,24,88,850,150,hwndMain,ID_INPUT);sendFont(hwndInput,normalFont)
	hwndTest=createWindow(0,"BUTTON","Load Test Items",WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON,24,248,150,34,hwndMain,ID_TESTITEMS)
	hwndClear=createWindow(0,"BUTTON","Clear",WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON,184,248,90,34,hwndMain,ID_CLEAR);sendFont(hwndTest,normalFont);sendFont(hwndClear,normalFont)
	h=createWindow(0,"STATIC","Output folder:",WS_CHILD|WS_VISIBLE,24,300,120,24,hwndMain,0);sendFont(h,normalFont)
	hwndOutput=createWindow(WS_EX_CLIENTEDGE,"EDIT",defaultOutputPath(),WS_CHILD|WS_VISIBLE|WS_TABSTOP|ES_AUTOHSCROLL,145,294,610,32,hwndMain,ID_OUTPUT)
	hwndBrowse=createWindow(0,"BUTTON","Browse...",WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON,765,294,109,32,hwndMain,ID_BROWSE);sendFont(hwndOutput,normalFont);sendFont(hwndBrowse,normalFont)
	hwndDownload=createWindow(0,"BUTTON","Download",WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON,24,345,150,40,hwndMain,ID_DOWNLOAD)
	hwndCancel=createWindow(0,"BUTTON","Cancel",WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON,184,345,110,40,hwndMain,ID_CANCEL)
	hwndOpen=createWindow(0,"BUTTON","Open Output",WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON,304,345,140,40,hwndMain,ID_OPEN)
	sendFont(hwndDownload,normalFont);sendFont(hwndCancel,normalFont);sendFont(hwndOpen,normalFont);enable(hwndCancel,false)
	hwndStatus=createWindow(0,"STATIC","Ready — BO3 AppID 311210",WS_CHILD|WS_VISIBLE,470,354,404,28,hwndMain,0);sendFont(hwndStatus,normalFont)
	h=createWindow(0,"STATIC","Activity",WS_CHILD|WS_VISIBLE,24,405,100,24,hwndMain,0);sendFont(h,normalFont)
	hwndLog=createWindow(WS_EX_CLIENTEDGE,"EDIT","",WS_CHILD|WS_VISIBLE|WS_VSCROLL|ES_LEFT|ES_MULTILINE|ES_AUTOVSCROLL|ES_READONLY,24,432,850,190,hwndMain,ID_LOG);sendFont(hwndLog,normalFont)
	h=createWindow(0,"STATIC","Standalone downloader • Valve SteamCMD • no porter / no PS4 SDK integration",WS_CHILD|WS_VISIBLE,24,635,700,24,hwndMain,0);sendFont(h,normalFont)
}
func defaultOutputPath()string{home,err:=os.UserHomeDir();if err!=nil||home==""{return filepath.Join(".","NEXUS_BO3_Workshop")};return filepath.Join(home,"Downloads","NEXUS_BO3_Workshop")}
func browseForFolder()string{
	display:=make([]uint16,260);title:=utf16Ptr("Choose where NEXUS should store BO3 Workshop items")
	bi:=browseInfo{HwndOwner:hwndMain,PszDisplayName:&display[0],LpszTitle:title,UlFlags:BIF_RETURNONLYFSDIRS|BIF_NEWDIALOGSTYLE}
	pidl,_,_:=pSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(&bi)));if pidl==0{return ""};defer pCoTaskMemFree.Call(pidl)
	path:=make([]uint16,32768);ok,_,_:=pSHGetPathFromIDListW.Call(pidl,uintptr(unsafe.Pointer(&path[0])));if ok==0{return ""};return syscall.UTF16ToString(path)
}
func setRunningUI(isRunning bool){enable(hwndDownload,!isRunning);enable(hwndInput,!isRunning);enable(hwndOutput,!isRunning);enable(hwndBrowse,!isRunning);enable(hwndTest,!isRunning);enable(hwndClear,!isRunning);enable(hwndCancel,isRunning)}
func startDownload(){
	if running.Swap(true){return}
	raw:=getText(hwndInput);ids,parseErrs:=parseWorkshopInputs(raw)
	if len(ids)==0{running.Store(false);messageBox(appTitle,"Paste at least one valid BO3 Workshop URL or numeric Workshop ID.",MB_OK|MB_ICONWARNING);return}
	output:=strings.TrimSpace(getText(hwndOutput))
	if output==""{running.Store(false);messageBox(appTitle,"Choose an output folder first.",MB_OK|MB_ICONWARNING);return}
	clearLog();appendLog("NEXUS BO3 Workshop Grabber v0.2");appendLog(fmt.Sprintf("BO3 AppID: %s",bo3AppID));appendLog(fmt.Sprintf("Queue: %d unique item(s)",len(ids)))
	for _,err:=range parseErrs{appendLog("Ignored input: "+err.Error())}
	appendLog("Output: "+output);appendLog("------------------------------------------------------------");setRunningUI(true)
	ctx,cancel:=context.WithCancel(context.Background());currentCancel=cancel
	go func(){
		err:=runBatch(ctx,ids,output,BatchCallbacks{Log:appendLog,Status:setStatus});currentCancel=nil;setRunningUI(false);running.Store(false)
		if err!=nil{if err==context.Canceled{setStatus("Cancelled.");appendLog("CANCELLED");return};setStatus("Stopped with an error.");appendLog("ERROR: "+err.Error());messageBox(appTitle,err.Error(),MB_OK|MB_ICONERROR);return}
		appendLog("------------------------------------------------------------");appendLog("ALL REQUESTED ITEMS COMPLETE");messageBox(appTitle,"Workshop download batch complete.",MB_OK|MB_ICONINFORMATION)
	}()
}
func windowProc(hwnd uintptr,message uint32,wParam,lParam uintptr)uintptr{
	switch message{
	case WM_CTLCOLOREDIT:pSetTextColor.Call(wParam,rgb(242,242,242));pSetBkColor.Call(wParam,rgb(24,24,24));return darkEditBrush
	case WM_CTLCOLORSTATIC:pSetTextColor.Call(wParam,rgb(242,242,242));pSetBkMode.Call(wParam,TRANSPARENT);return darkBgBrush
	case WM_CTLCOLORBTN:pSetTextColor.Call(wParam,rgb(242,242,242));pSetBkMode.Call(wParam,TRANSPARENT);return darkBgBrush
	case WM_COMMAND:
		id:=uint16(wParam&0xffff)
		switch id{
		case ID_BROWSE:if p:=browseForFolder();p!=""{setText(hwndOutput,p)}
		case ID_TESTITEMS:setText(hwndInput,"https://steamcommunity.com/sharedfiles/filedetails/?id=2801151115\r\nhttps://steamcommunity.com/sharedfiles/filedetails/?id=1795684736")
		case ID_CLEAR:setText(hwndInput,"");clearLog();setStatus("Ready — BO3 AppID 311210")
		case ID_DOWNLOAD:startDownload()
		case ID_CANCEL:if currentCancel!=nil{currentCancel();setStatus("Cancelling...")}
		case ID_OPEN:path:=strings.TrimSpace(getText(hwndOutput));if path==""{path=defaultOutputPath()};_=os.MkdirAll(path,0o755);_=exec.Command("explorer.exe",path).Start()
		};return 0
	case WM_DESTROY:if currentCancel!=nil{currentCancel()};pPostQuitMessage.Call(0);return 0
	}
	ret,_,_:=pDefWindowProcW.Call(hwnd,uintptr(message),wParam,lParam);return ret
}
func main(){
	runtime.LockOSThread();defer runtime.UnlockOSThread();pSetProcessDPIAware.Call();pCoInitializeEx.Call(0,COINIT_APARTMENTTHREADED);defer pCoUninitialize.Call()
	hInstance,_,_:=pGetModuleHandleW.Call(0);cursor,_,_:=pLoadCursorW.Call(0,IDC_ARROW)
	darkBgBrush,_,_=pCreateSolidBrush.Call(rgb(10,10,10));darkEditBrush,_,_=pCreateSolidBrush.Call(rgb(24,24,24))
	className:=utf16Ptr("NEXUS_BO3_WORKSHOP_GRABBER")
	wc:=wndClassEx{CbSize:uint32(unsafe.Sizeof(wndClassEx{})),LpfnWndProc:syscall.NewCallback(windowProc),HInstance:hInstance,HCursor:cursor,HbrBackground:darkBgBrush,LpszClassName:className}
	atom,_,err:=pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)));if atom==0{_=err;return}
	hwndMain=createWindow(0,"NEXUS_BO3_WORKSHOP_GRABBER",appTitle,WS_OVERLAPPEDWINDOW,120,80,920,715,0,0);if hwndMain==0{return}
	applyDarkTitleBar(hwndMain);applyDarkTheme(hwndMain);createUI();pShowWindow.Call(hwndMain,SW_SHOW);pUpdateWindow.Call(hwndMain)
	var m msg
	for{r,_,_:=pGetMessageW.Call(uintptr(unsafe.Pointer(&m)),0,0,0);if int32(r)<=0{break};pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)));pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))}
}
