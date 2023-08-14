package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Please provide a domain as the first argument.")
		return
	}

	domain1, err := url.Parse(os.Args[1])
	if err != nil || domain1.Scheme == "" || domain1.Host == "" {

		fmt.Println("Please provide a domain as the first argument.")
		return
	}
	domain := domain1.String()

	fmt.Println("Received domain:", domain)
	// 配置选项，禁用headless模式，使得浏览器可见
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),    // 关闭headless模式
		chromedp.Flag("disable-gpu", false), // 若在非headless模式下遇到问题，尝试关闭此选项
	)

	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
	//defer cancel()

	ctx, _ := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	//defer cancel()
	//domain := `https://www.mingw-w64.org/`
	// 开启网络监听
	//done := make(chan bool)
	var secondaryDomains = make(map[string]struct{})

	requests := make(map[network.RequestID]string)
	var waitGroup sync.WaitGroup
	//var id network.RequestID
	//requests := make(map[network.RequestID]string)
	var domainMutex sync.Mutex
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch event := ev.(type) {
		case *network.EventResponseReceived:
			// 当接收到一个新的响应时，保存其 URL
			requests[event.RequestID] = event.Response.URL
			fmt.Println("Data1 for:", event.Response.URL)
		case *network.EventLoadingFinished:

			if url_c, exists := requests[event.RequestID]; exists {
				waitGroup.Add(1)
				go func(url_c string, requestID network.RequestID, domain_g string) {
					defer waitGroup.Done()
					domain_gc, _ := url.Parse(domain_g)
					primaryDomain := domain_gc.Hostname() // 替换为您的主域名

					if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
						data, err := network.GetResponseBody(requestID).Do(ctx)
						if err != nil {
							return err
						}

						// 解析URL
						parsedURL, err := url.Parse(url_c)
						if err != nil {
							return err
						}

						// 根据是否为主域名确定根目录
						rootPath := primaryDomain
						if parsedURL.Hostname() != primaryDomain {
							rootPath = filepath.Join(primaryDomain, parsedURL.Hostname())
							domainMutex.Lock()
							secondaryDomains[parsedURL.Hostname()] = struct{}{}
							domainMutex.Unlock()
						}

						// 创建目录结构
						dirPath := filepath.Join(rootPath, filepath.Dir(parsedURL.Path))
						os.MkdirAll(dirPath, 0755)

						filename := strings.Split(filepath.Base(parsedURL.Path), "?")[0]

						// 如果URL是根路径，则将文件名设置为index.html
						if url_c == domain || parsedURL.Path == "" || parsedURL.Path == "/" || parsedURL.Path == "\\" {

							filename = "index.html"

						}

						filePath := filepath.Join(dirPath, filename)

						err = os.WriteFile(filePath, data, 0644)
						if err != nil {
							return err
						}

						fmt.Println("Saved data for:", url_c, "to", filePath)
						return nil
					})); err != nil {
						fmt.Println("Error:", err)
					}
				}(url_c, event.RequestID, domain)

			}
		}
	})

	// 这里添加你的其他chromedp任务...

	// 执行任务
	err = chromedp.Run(ctx, chromedp.Navigate(domain))
	if err != nil {
		log.Fatal(err)
	}
	waitGroup.Wait()
	// 确保所有下载任务都已完成
	domain_gg, _ := url.Parse(domain)
	domain_ggg := domain_gg.Hostname()
	// 递归遍历文件的函数
	var walkFiles func(path string) error
	walkFiles = func(path string) error {
		files, err := os.ReadDir(path)
		if err != nil {
			return err
		}

		for _, f := range files {
			fullPath := filepath.Join(path, f.Name())
			if f.IsDir() {
				walkFiles(fullPath)
			} else {
				content, err := os.ReadFile(fullPath)
				if err != nil {
					return err
				}
				modifiedContent := string(content)

				//modifiedContent = strings.ReplaceAll(path, mainDomainWithHTTPS, "/")
				// 对每个secondaryDomains进行检查
				modifiedContent = strings.ReplaceAll(modifiedContent, domain_ggg, "./")
				for domain := range secondaryDomains {
					if strings.Contains(domain, domain_ggg) {
						oldURL := "https://" + domain
						modifiedContent = strings.ReplaceAll(modifiedContent, oldURL, "./")
						modifiedContent = strings.ReplaceAll(modifiedContent, domain_ggg, "./")
						oldURLWithoutS := "http://" + domain
						modifiedContent = strings.ReplaceAll(modifiedContent, oldURLWithoutS, "./")

					} else {
						oldURL := "https://" + domain
						modifiedContent = strings.ReplaceAll(modifiedContent, oldURL, "./"+domain+"/")
						oldURLWithoutS := "http://" + domain
						modifiedContent = strings.ReplaceAll(modifiedContent, oldURLWithoutS, "./"+domain+"/")
					}

				}

				// 如果有更改，写回到文件
				if modifiedContent != string(content) {
					err = os.WriteFile(fullPath, []byte(modifiedContent), 0644)
					if err != nil {
						return err
					}
				}
			}
		}

		return nil
	}

	// 从主域名目录开始遍历
	err = walkFiles(domain_ggg)
	if err != nil {
		fmt.Println("Error:", err)
	}

}
