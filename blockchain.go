package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	_ "github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"io"
	"log"
	"net"
	_ "net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

//////////////////// 处理区块链 ////////////////////
const difficulty = 3 // 定义难度，也就是哈希包含多少个0的前缀
type Block struct {
	Index int // 表示区块所在区块链的位置
	Timestamp string // 生成区块的时间戳
	Data int // 写入区块的数据
	Hash string // 整个区块数据SHA256的哈希
	PrevHash string // 上一个区块的哈希值
	Difficulty int // 定义难度
	Nonce string // 定义一个Nonce
}
var mutex = &sync.Mutex{} // 防止并发写入请求造成的错误，加入互斥锁
var BlockChain []Block // 定义一个区块链，数据元素要全部都是Block
var bcServer chan []Block // 定义一个channel，处理各个节点之间的同步问题
/**
计算区块哈希值
*/
func calculateHash(block Block) string {
	record := strconv.Itoa(block.Index) + block.Timestamp + strconv.Itoa(block.Data) + block.PrevHash + block.Nonce // 得到当前block区块的字符串拼接，按照索引、时间戳、所含数据、上一个区块哈希来进行记录，Nonce值一并加入
	h := sha256.New() // 得到sha256哈希算法
	h.Write([]byte(record)) // 得到对应哈希
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed) //转化为字符串返回
}
/**
生成一个区块，根据上一个区块
*/
func generateBlock(oldBlock Block, Data int) (Block, error) {
	var newBlock Block
	t := time.Now()
	newBlock.Index = oldBlock.Index + 1 // 索引自增
	newBlock.Timestamp = t.String() // 时间戳
	newBlock.Data = Data // 数据
	newBlock.PrevHash = oldBlock.Hash // 上一个区块的哈希
	newBlock.Difficulty = difficulty // 难度
	//newBlock.Hash = calculateHash(newBlock) // 计算本区块的哈希
	for i := 0; ; i++ {
		hex := fmt.Sprintf("%x", i) // 16进制展示
		newBlock.Nonce = hex
		newHash := calculateHash(newBlock) // 计算哈希
		if !isHashValid(newHash, newBlock.Difficulty) {
			//fmt.Println(newHash, " 继续努力！🆙")
			time.Sleep(time.Millisecond) // 每隔1s执行一次
			continue
		} else {
			fmt.Println(newHash, " 已经成功！")
			newBlock.Hash = newHash
			break
		}
	}
	return newBlock, nil
}

/**
验证区块是否合法
*/
func isBlockValid(newBlock, oldBlock Block) bool {
	if oldBlock.Index + 1 != newBlock.Index { // 如果索引不继承自上一个，验证不通过
		return false
	}
	if oldBlock.Hash != newBlock.PrevHash { // 如果哈希不继承上一个区块，验证不通过
		return false
	}
	if calculateHash(newBlock) != newBlock.Hash { // 如果计算出来的哈希不一致，验证不通过
		return false
	}
	return true
}
/**
验证哈希的前缀是否包含difficulty个0
*/
func isHashValid(hash string, difficulty int) bool {
	prefix := strings.Repeat("0", difficulty)
	return strings.HasPrefix(hash, prefix)
}

/**
选择长链作为正确的链
*/
func replaceChain(newBlocks []Block) {
	if len(newBlocks) > len(BlockChain) { // 计算数组长度
		BlockChain = newBlocks
	}
}

////////////////// 主函数 /////////////////

func main () {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	bcServer = make(chan []Block) // 创建通道

	t := time.Now()
	genesisBlock := Block{0, t.String(), 0, "", "", difficulty, ""}
	spew.Dump(genesisBlock)
	BlockChain = append(BlockChain, genesisBlock) // 创世区块

	server, err := net.Listen("tcp", ":" + os.Getenv("PORT")) // 监听TCP端口
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close() // 完成后关闭server

	for {
		conn, err := server.Accept()
		if err != nil {
			log.Fatal()
		}
		go handleConn(conn) // 协程处理连接
	}
}
/**
处理连接
*/
func handleConn(conn net.Conn) {
	defer conn.Close() // 完成后关闭
	spew.Dump(conn)
	_, _ = io.WriteString(conn, "输入数字：")
	scanner := bufio.NewScanner(conn)

	go func() {
		for scanner.Scan() { // 轮询扫描所有tcp连接
			data, err := strconv.Atoi(scanner.Text())
			var newBlock Block

			if err != nil {
				log.Printf("%v 非数字 %s\n", scanner.Text(), err)
				goto END
			} else {
				log.Printf("Input: %v\n", data)
			}
			newBlock, err = generateBlock(BlockChain[len(BlockChain) - 1], data)

			if err != nil {
				log.Println(err)
				goto END
			}

			if isBlockValid(newBlock, BlockChain[len(BlockChain) - 1]) {
				newBlockChain := append(BlockChain, newBlock)
				replaceChain(newBlockChain)
				bcServer <- BlockChain // 将生成的区块数据交给通道，单向传递
			} else {
				io.WriteString(conn, "Invalid new block\n")
				goto END
			}

			END: io.WriteString(conn, "输入数字：\n")
		}
	}()

	go func() {
		var currentBlockChain string

		for { // 每隔10s同步一次
			time.Sleep(10 * time.Second)
			output, err := json.MarshalIndent(BlockChain, "", " ")

			if err != nil {
				log.Fatal(err)
				continue
			}
			strOutput := string(output)
			if currentBlockChain == strOutput {
				continue
			} else {
				io.WriteString(conn, "\n↓↓↓↓↓↓↓↓↓↓↓↓↓ 同步区块链：↓↓↓↓↓↓↓↓↓↓↓↓↓↓\n"+ strOutput + "\n↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑↑\n")
				currentBlockChain = strOutput
			}
		}
	}()

	for _= range bcServer {
		spew.Dump(BlockChain)
	}
}