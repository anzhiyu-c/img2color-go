/*
 * @Description: 
 * @Author: 安知鱼
 * @Email: anzhiyu-c@qq.com
 * @Date: 2023-07-26 15:25:07
 * @LastEditTime: 2023-07-26 17:00:04
 * @LastEditors: 安知鱼
 */
package handler
 
import (
  "fmt"
  "net/http"
)
 
func Handler(w http.ResponseWriter, r *http.Request) {
  fmt.Fprintf(w, "<h1>Hello from Go!</h1>")
}