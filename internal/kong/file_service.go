package kong

import (
	"context"
	"fmt"
	"github.com/kong/go-kong/kong"
	"strings"
)

// AbstractFileService handles Files in Kong.
type AbstractFileService interface {
	// Create creates a File in Kong.
	Create(ctx context.Context, file *File) (*File, error)
	// Get fetches a File in Kong.
	Get(ctx context.Context, file *File) (*File, error)
	// Update updates a File in Kong
	Update(ctx context.Context, file *File) (*File, error)
	// Delete deletes a File in Kong
	Delete(ctx context.Context, file *File) error
}

// FileService handles Files in Kong.
type FileService struct {
	client *kong.Client
}

// Is empty
func isEmptyString(s *string) bool {
	return s == nil || strings.TrimSpace(*s) == ""
}

func NewFileService(kongClient *kong.Client) (fileService FileService) {

	return FileService{
		client: kongClient,
	}
}

// Create creates a File in Kong.
// If an ID is specified, it will be used to
// create a file in Kong, otherwise an ID
// is auto-generated.
func (s *FileService) Create(ctx context.Context,
	file *File) (*File, error) {

	endpoint := fmt.Sprintf("/files/%v", *file.Path)
	req, err := s.client.NewRequest("PUT", endpoint, nil, file)
	if err != nil {
		return nil, err
	}

	var response File
	_, err = s.client.Do(ctx, req, &response)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// Get fetches a File in Kong.
func (s *FileService) Get(ctx context.Context, file *File) (*File, error) {

	if isEmptyString(file.Path) {
		return nil, fmt.Errorf("Path cannot be nil for Get operation")
	}

	endpoint := fmt.Sprintf("/files/%v", *file.Path)
	req, err := s.client.NewRequest("GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}

	var response File
	_, err = s.client.Do(ctx, req, &response)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// Update updates a File in Kong
func (s *FileService) Update(ctx context.Context, file *File) (*File, error) {

	if isEmptyString(file.Path) {
		return nil, fmt.Errorf("Path cannot be nil for Update operation")
	}

	endpoint := fmt.Sprintf("/files/%v", *file.Path)
	req, err := s.client.NewRequest("PUT", endpoint, nil, file)
	if err != nil {
		return nil, err
	}

	var response File
	_, err = s.client.Do(ctx, req, &response)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// Delete deletes a File in Kong
func (s *FileService) Delete(ctx context.Context, file *File) (*File, error) {

	if isEmptyString(file.Path) {
		return file, fmt.Errorf("Path cannot be nil for Delete operation")
	}

	endpoint := fmt.Sprintf("/files/%v", *file.Path)
	req, err := s.client.NewRequest("DELETE", endpoint, nil, nil)
	if err != nil {
		return file, err
	}

	_, err = s.client.Do(ctx, req, nil)
	return file, err
}
