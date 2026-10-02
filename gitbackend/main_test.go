package gitbackend

import (
	"os"
	"testing"
)

// TestMain 抹平宿主机环境对夹具提交身份的污染。
//
// 部分开发机/调出会导出 GIT_AUTHOR_NAME / GIT_COMMITTER_* 等环境变量，
// 其优先级高于仓库本地 config（createTestRepo 设的 user.name=Test 会被
// 覆盖），使依赖 Author=="Test" 的断言在本机红、在无此环境的 CI 绿——
// 典型的环境决定型测试失败。这里在进程级注入恒定身份：子进程 git 与
// backend 的 native 执行都继承它；backend 显式带身份的路径
// （CommitWithIdentity 等）以自身注入的 env 覆盖本值，不受影响。
func TestMain(m *testing.M) {
	os.Setenv("GIT_AUTHOR_NAME", "Test")
	os.Setenv("GIT_AUTHOR_EMAIL", "test@test.com")
	os.Setenv("GIT_COMMITTER_NAME", "Test")
	os.Setenv("GIT_COMMITTER_EMAIL", "test@test.com")
	os.Exit(m.Run())
}
