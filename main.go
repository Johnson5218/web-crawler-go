package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"golang.org/x/net/html"
)

func forEachNode(n *html.Node, pre, post func(n *html.Node)) {
	if pre != nil {
		pre(n)
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		forEachNode(c, pre, post)
	}

	if post != nil {
		post(n)
	}
}

var depth int

func startElement(n *html.Node) {
	if n.Type == html.ElementNode {
		fmt.Printf("%*s<%s>\n", depth*2, "", n.Data)
		depth++
	}
}

func endElement(n *html.Node) {
	if n.Type == html.ElementNode {
		depth--
		fmt.Printf("%*s</%s>\n", depth*2, "", n.Data)
	}
}

func Extract(url string) ([]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("getting %s: %s", url, resp.Status)
	}
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing %s as HTML: %v", url, err)
	}
	var links []string
	visitNode := func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key != "href" {
					continue
				}
				// 解析为基于当前文档的相对路径 resp.Request.URL
				link, err := resp.Request.URL.Parse(a.Val)
				if err != nil {
					continue // 忽略不合法的 URL
				}
				links = append(links, link.String())
			}
		}
	}
	forEachNode(doc, visitNode, nil)
	return links, nil
}

// breadthFirst 对每个 worklist 元素调用 f
// 并将返回的内容添加到 worklist 中，对每个元素，最多调用一次 f
func breadthFirst(f func(item string) []string, worklist []string) {
	seen := make(map[string]bool)
	for len(worklist) > 0 {
		items := worklist
		worklist = nil
		for _, item := range items {
			if !seen[item] {
				seen[item] = true
				worklist = append(worklist, f(item)...)
			}
		}
	}
}

// 非并发版本
//func crawl(url string) []string {
//	fmt.Println(url)
//	list, err := Extract(url)
//	if err != nil {
//		log.Print(err)
//	}
//	return list
//}

// 并发版本 V0.90
// 可以使用容量为n的缓冲通道来建立一个并发原语，成为计数信号量
// 概念上，对于缓冲通道中的n个空闲槽，每一个代表一个令牌，持有者可以执行
// 令牌是一个计数信号量
// 确保并发请求限制在20个以内
var tokens = make(chan struct{}, 20)

func crawl(url string) []string {
	fmt.Println(url)
	tokens <- struct{}{} // 获取令牌
	list, err := Extract(url)
	<-tokens // 释放令牌
	if err != nil {
		log.Print(err)
	}
	return list
}

// 整个过程将在所有可到达的网页被访问到或者内存耗尽时结束
func main() {
	// 开始广度遍历
	// 从命令行参数开始
	//breadthFirst(crawl, os.Args[1:])

	// 并发版本 V0.90
	//worklist := make(chan []string)
	//// 从命令行参数开始
	//go func() { worklist <- os.Args[1:] }()
	//
	//// 并发爬取 Web
	//seen := make(map[string]bool)
	//for list := range worklist {
	//	for _, link := range list {
	//		if !seen[link] {
	//			seen[link] = true
	//			go func(link string) {
	//				worklist <- crawl(link)
	//			}(link)
	//		}
	//	}
	//}

	//
	worklist := make(chan []string)
	var n int // 等待发送到任务列表的数量

	// 从命令行参数开始
	n++
	go func() { worklist <- os.Args[1:] }()

	// 并发获取爬取Web
	seen := make(map[string]bool)
	for ; n > 0; n-- {
		list := <-worklist
		for _, link := range list {
			if !seen[link] {
				seen[link] = true
				n++
				go func(link string) {
					worklist <- crawl(link)
				}(link)
			}
		}
	}

	// 替代方案
	//worklist := make(chan []string)  // 可能有重复的URL列表
	//unseenLinks := make(chan string) // 去重后的URL列表
	//
	//// 向任务列表中添加命令行参数
	//go func() { worklist <- os.Args[1:] }()
	//
	//// 创建20个爬虫goroutine来获取每个不可见链接
	//for i := 0; i < 20; i++ {
	//	go func() {
	//		for link := range unseenLinks {
	//			foundLinks := crawl(link)
	//			go func() { worklist <- foundLinks }()
	//		}
	//	}()
	//}
	//
	//// 主goroutine 对URL列表进行去重
	//// 并把没有爬取过的条目发送给爬虫程序
	//seen := make(map[string]bool)
	//for list := range worklist {
	//	for _, link := range list {
	//		if !seen[link] {
	//			seen[link] = true
	//			unseenLinks <- link
	//		}
	//	}
	//}
}
