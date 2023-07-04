package main

import (
	"EH-Proxy/pkg/proxy"
	"EH-Proxy/pkg/system/sysPrint"
	"fmt"
	"sync"
)

func main() {
	p := proxy.GetProxyInstance()
	pm := proxy.GetProxyManagerInstance()
	var wg sync.WaitGroup
	fmt.Println(`
 ______     __  __     ______   ______     ______     __  __     __  __    
/\  ___\   /\ \_\ \   /\  == \ /\  == \   /\  __ \   /\_\_\_\   /\ \_\ \   
\ \  __\   \ \  __ \  \ \  _-/ \ \  __<   \ \ \/\ \  \/_/\_\/_  \ \____ \  
 \ \_____\  \ \_\ \_\  \ \_\    \ \_\ \_\  \ \_____\   /\_\/\_\  \/\_____\ 
  \/_____/   \/_/\/_/   \/_/     \/_/ /_/   \/_____/   \/_/\/_/   \/_____/ 
                                                                           
`)
	wg.Add(2)
	go func() {
		p.Serve()
		wg.Done()
	}()
	go func() {
		pm.Serve()
		wg.Done()
	}()
	wg.Wait()
	sysPrint.PrintlnSystemMsg("EH-Proxy is now ready to exit, bye bye...")
}
