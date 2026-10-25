package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"golang.org/x/sys/windows"
)

func TestNativeSmoke(t *testing.T) {
	if os.Getenv("TOOLS_SWITCHER_NATIVE_TEST") != "1" || runtime.GOOS != "windows" {
		t.Skip("set TOOLS_SWITCHER_NATIVE_TEST=1 on Windows")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole := windows.NewLazySystemDLL("ole32.dll")
	resultCode, _, _ := ole.NewProc("CoInitializeEx").Call(0, 2)
	if int32(resultCode) < 0 {
		t.Fatalf("CoInitializeEx: %x", resultCode)
	}
	defer ole.NewProc("CoUninitialize").Call()
	t.Setenv("TOOLS_SWITCHER_CONFIG_DIR", t.TempDir())
	app, window, err := newApplication()
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Private-Network", "true")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		var report struct {
			Result string `json:"result"`
		}
		_ = json.NewDecoder(r.Body).Decode(&report)
		select {
		case result <- report.Result:
		default:
		}
		w.WriteHeader(200)
		go app.Quit()
	}))
	defer server.Close()
	address, _ := json.Marshal(server.URL)
	script := `(async()=>{
  const report=async result=>{await fetch(` + string(address) + `,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({result})})};
  const wait=async condition=>{for(let i=0;i<900;i++){if(condition())return;await new Promise(resolve=>setTimeout(resolve,50))}throw new Error('UI timeout: '+document.body.innerText)};
  const click=text=>{const button=[...document.querySelectorAll('button')].find(b=>b.textContent.trim().endsWith(text));if(!button)throw new Error('Missing button '+text);button.click()};
  try{
   await wait(()=>document.body.innerText.includes('Java Development Kits'));
   click('Repositorios');await wait(()=>document.body.innerText.includes('Validar y añadir'));
   click('+ Validar y añadir');await wait(()=>[...document.querySelectorAll('h3')].some(h=>h.textContent==='Eclipse Temurin'));
   click('JDKs');await wait(()=>document.body.innerText.includes('Java Development Kits'));click('+ Instalar');
   await wait(()=>[...document.querySelectorAll('.package-row')].some(row=>/Eclipse Temurin 21\./.test(row.textContent)));
   await report('ok');
  }catch(error){await report(String(error))}
 })()`
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) { window.ExecJS(script) })
	timeout := time.AfterFunc(65*time.Second, func() {
		select {
		case result <- "native test timeout":
		default:
		}
		app.Quit()
	})
	defer timeout.Stop()
	if err = app.Run(); err != nil {
		t.Fatal(err)
	}
	select {
	case report := <-result:
		if report != "ok" {
			t.Fatal(report)
		}
	default:
		t.Fatal("application exited without report")
	}
}
