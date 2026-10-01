package gitbackend

import (
	"context"
	"strings"
)

// --- Remote operations ---

func (b *NativeGitBackend) AddRemote(ctx context.Context, repoPath, name, url string) error {
	_, stderr, err := b.runGit(ctx, repoPath, []string{"remote", "add", name, url}, AuthConfig{})
	if err != nil {
		return newGitError("AddRemote", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) RemoveRemote(ctx context.Context, repoPath, name string) error {
	_, stderr, err := b.runGit(ctx, repoPath, []string{"remote", "remove", name}, AuthConfig{})
	if err != nil {
		return newGitError("RemoveRemote", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) GetRemoteURL(ctx context.Context, repoPath, name string) (string, error) {
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"remote", "get-url", name}, AuthConfig{})
	if err != nil {
		return "", newGitError("GetRemoteURL", repoPath, stderr, err)
	}
	return strings.TrimSpace(stdout), nil
}

func (b *NativeGitBackend) GetRemotes(ctx context.Context, repoPath string) ([]string, error) {
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"remote"}, AuthConfig{})
	if err != nil {
		return nil, newGitError("GetRemotes", repoPath, stderr, err)
	}
	var names []string
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

// --- Tag operations ---

func (b *NativeGitBackend) CreateTag(ctx context.Context, repoPath, name, ref string) error {
	args := []string{"tag", name}
	if ref != "" {
		args = append(args, ref)
	}
	_, stderr, err := b.runGit(ctx, repoPath, args, AuthConfig{})
	if err != nil {
		if strings.Contains(stderr, "already exists") {
			return newGitError("CreateTag", repoPath, stderr, ErrTagExists)
		}
		return newGitError("CreateTag", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) DeleteTag(ctx context.Context, repoPath, name string) error {
	_, stderr, err := b.runGit(ctx, repoPath, []string{"tag", "-d", name}, AuthConfig{})
	if err != nil {
		return newGitError("DeleteTag", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) PushTag(ctx context.Context, repoPath, remote, name string, auth AuthConfig) error {
	_, stderr, err := b.runGit(ctx, repoPath, []string{"push", remote, "refs/tags/" + name}, auth)
	if err != nil {
		return newGitError("PushTag", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) GetTagList(ctx context.Context, repoPath string) ([]TagInfo, error) {
	// 字段分隔用 TAB，subject 置于末位：标签消息是任意文本，含分隔符时
	// 由 SplitN 的最后一个分段整体吸收（for-each-ref 类 format 不支持
	// %x00，argv 也无法内嵌 NUL，TAB 是能做到的最强分隔）。
	// Author 字段：annotated tag 的身份在 tagger（%(authorname) 恒空），
	// lightweight tag 在 commit author——条件格式取先命中者（旧实现
	// 对 annotated tag 的 Author 恒为空串，属顺带修复）。
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{
		"tag", "-l", "--format=%(refname:short)\t%(objectname)\t%(if)%(taggername)%(then)%(taggername)%(else)%(authorname)%(end)\t%(subject)",
	}, AuthConfig{})
	if err != nil {
		return nil, newGitError("GetTagList", repoPath, stderr, err)
	}

	var tags []TagInfo
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 4 {
			continue
		}
		tags = append(tags, TagInfo{
			Name:    parts[0],
			Hash:    parts[1],
			Author:  parts[2],
			Message: parts[3],
		})
	}
	return tags, nil
}
