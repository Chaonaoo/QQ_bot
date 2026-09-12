package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Store 画像存储（内存 + JSON 文件持久化）
type Store struct {
	mu      sync.RWMutex
	profile *Profile
	dataDir string
}

// NewStore 创建画像存储实例
func NewStore(dataDir string) *Store {
	return &Store{dataDir: dataDir}
}

// Save 将画像保存到内存和文件
func (s *Store) Save(p *Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.profile = p

	// 持久化到文件
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	filePath := filepath.Join(s.dataDir, p.QQ+".json")
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化画像失败: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("写入画像文件失败: %w", err)
	}
	return nil
}

// Get 获取当前画像（内存优先，否则从文件加载）
func (s *Store) Get(qq string) (*Profile, error) {
	s.mu.RLock()
	if s.profile != nil && s.profile.QQ == qq {
		defer s.mu.RUnlock()
		return s.profile, nil
	}
	s.mu.RUnlock()

	// 尝试从文件加载
	filePath := filepath.Join(s.dataDir, qq+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("画像文件不存在: %s", filePath)
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("解析画像文件失败: %w", err)
	}

	s.mu.Lock()
	s.profile = &p
	s.mu.Unlock()

	return &p, nil
}
