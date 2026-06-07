package client

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"
)

// UpdateUsername changes the username of the current user/bot.
// Pass an empty string to remove the username.
func (c *MCUBClient) UpdateUsername(ctx context.Context, username string) error {
	_, err := c.api.AccountUpdateUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("update username: %w", err)
	}
	return nil
}

// UpdateBio updates the bio/about text of the current user.
func (c *MCUBClient) UpdateBio(ctx context.Context, bio string) error {
	req := &tg.AccountUpdateProfileRequest{
		About: bio,
	}
	req.Flags.Set(2) // set the About flag
	_, err := c.api.AccountUpdateProfile(ctx, req)
	if err != nil {
		return fmt.Errorf("update bio: %w", err)
	}
	return nil
}

// UpdateFirstName updates the first and last name of the current user.
// Pass an empty lastName to clear it.
func (c *MCUBClient) UpdateFirstName(ctx context.Context, firstName, lastName string) error {
	req := &tg.AccountUpdateProfileRequest{
		FirstName: firstName,
		LastName:  lastName,
	}
	req.Flags.Set(0) // FirstName flag
	req.Flags.Set(1) // LastName flag
	_, err := c.api.AccountUpdateProfile(ctx, req)
	if err != nil {
		return fmt.Errorf("update name: %w", err)
	}
	return nil
}

// UpdateProfilePhoto uploads a local image file and sets it as the profile photo.
func (c *MCUBClient) UpdateProfilePhoto(ctx context.Context, filePath string) error {
	uploaded, err := c.uploadFile(ctx, UploadParams{Path: filePath})
	if err != nil {
		return fmt.Errorf("upload photo: %w", err)
	}

	_, err = c.api.PhotosUploadProfilePhoto(ctx, &tg.PhotosUploadProfilePhotoRequest{
		File: uploaded.InputFile,
	})
	if err != nil {
		return fmt.Errorf("set profile photo: %w", err)
	}
	return nil
}

// DeleteProfilePhoto removes the current user's profile photo.
// It fetches the most recent photo and deletes it.
func (c *MCUBClient) DeleteProfilePhoto(ctx context.Context) error {
	photos, err := c.GetProfilePhotos(ctx, 0, 1)
	if err != nil {
		return fmt.Errorf("get photos for deletion: %w", err)
	}
	if len(photos) == 0 {
		return nil // nothing to delete
	}

	inputPhotos := make([]tg.InputPhotoClass, 0, len(photos))
	for _, p := range photos {
		inputPhotos = append(inputPhotos, &tg.InputPhoto{
			ID:            p.ID,
			AccessHash:    p.AccessHash,
			FileReference: p.FileReference,
		})
	}

	_, err = c.api.PhotosDeletePhotos(ctx, inputPhotos)
	if err != nil {
		return fmt.Errorf("delete profile photo: %w", err)
	}
	return nil
}
