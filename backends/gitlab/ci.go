package gitlab

import (
	"bufio"
	"context"
	"io"

	gitlab "gitlab.com/gitlab-org/api/client-go/v3"

	"github.com/yi-nology/go-git-platform/provider"
)

// ciTraceTailBytes 失败日志尾部保留量：CI 失败的根因几乎总在日志尾部
// （报错与堆栈），头部只有无关的 setup 输出。20KB 对 LLM 归因足够。
const ciTraceTailBytes = 20_000

// ListFailedJobLogs implements provider.CILogManager（可选能力，仅 GitLab）：
// 最近一条 pipeline 的失败 job → trace 尾部。无 pipeline / 全绿 → 空切片。
func (p *Provider) ListFailedJobLogs(ctx context.Context, owner, repo, sha string) ([]provider.JobLog, error) {
	pid := pidOf(owner, repo)
	shaOpt := sha
	pl, _, err := p.client.Pipelines.ListProjectPipelines(pid, &gitlab.ListProjectPipelinesOptions{
		SHA:     &shaOpt,
		OrderBy: new("id"),
		Sort:    new("desc"),
		ListOptions: gitlab.ListOptions{
			PerPage: 1,
		},
	}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "ListProjectPipelines", err)
	}
	if len(pl) == 0 {
		return nil, nil
	}
	jobs, _, err := p.client.Jobs.ListPipelineJobs(pid, pl[0].ID, &gitlab.ListJobsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "ListPipelineJobs", err)
	}
	out := make([]provider.JobLog, 0, len(jobs))
	for _, j := range jobs {
		// canceled 的 job 是被连带取消的，不产生可归因日志；created 的没轮到
		// 跑——只收真正跑挂了的。
		if j.Status != "failed" {
			continue
		}
		tail, truncated, terr := p.traceTail(ctx, pid, j.ID)
		if terr != nil {
			// 单 job trace 拉不到不阻塞其余 job 的归因
			tail, truncated = "", false
		}
		out = append(out, provider.JobLog{
			Name: j.Name, Stage: j.Stage, WebURL: j.WebURL,
			Trace: tail, Truncat: truncated,
		})
	}
	return out, nil
}

// traceTail 拉 job trace 并只留尾部（GetTraceFile 返回全文流）。
func (p *Provider) traceTail(ctx context.Context, pid string, jobID int64) (string, bool, error) {
	r, _, err := p.client.Jobs.GetTraceFile(pid, jobID, gitlab.WithContext(ctx))
	if err != nil {
		return "", false, provider.Wrap(provider.PlatformGitLab, "GetTraceFile", err)
	}
	return tailOfReader(r, ciTraceTailBytes)
}

// tailOfReader 读流并保留尾部 budget 字节（行粒度扫描避免整文驻留内存；
// trace 可能数十 MB）。
func tailOfReader(r io.Reader, budget int) (string, bool, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	var lines []string
	total := 0
	for sc.Scan() {
		lines = append(lines, sc.Text())
		total += len(sc.Text()) + 1
		for total > budget*4 { // 行数软上限：超 4 倍预算从头部丢弃
			total -= len(lines[0]) + 1
			lines = lines[1:]
		}
	}
	if err := sc.Err(); err != nil {
		return "", false, err
	}
	joined := ""
	for i := len(lines) - 1; i >= 0; i-- {
		if joined == "" {
			joined = lines[i]
			continue
		}
		if len(joined)+len(lines[i])+1 > budget {
			return joined, true, nil
		}
		joined = lines[i] + "\n" + joined
	}
	return joined, false, nil
}
