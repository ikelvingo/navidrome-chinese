package cmd

import (
	"context"
	"fmt"
	"os"

	. "github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/core/artwork"
	"github.com/navidrome/navidrome/db"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/spf13/cobra"
)

var (
	clearImages   bool
	clearBio      bool
	clearSimilar  bool
	clearAll      bool
	artistID      string
	artistName    string
	refreshAlbums bool
)

func init() {
	refreshCmd.Flags().StringVar(&artistID, "id", "", "Artist ID to refresh")
	refreshCmd.Flags().StringVar(&artistName, "name", "", "Artist name to refresh (partial match)")
	refreshCmd.Flags().BoolVar(&clearImages, "clear-images", false, "Clear image URLs")
	refreshCmd.Flags().BoolVar(&clearBio, "clear-bio", false, "Clear biography")
	refreshCmd.Flags().BoolVar(&clearSimilar, "clear-similar", false, "Clear similar artists")
	refreshCmd.Flags().BoolVar(&clearAll, "clear-all", false, "Clear all external info (images, bio, similar artists)")
	refreshCmd.Flags().BoolVar(&refreshAlbums, "albums", false, "Also refresh albums for the artist")
	rootCmd.AddCommand(refreshCmd)
}

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh external artist/album information",
	Long: `Force refresh external information for artists or albums.
This will clear the cached external info so it will be re-fetched from external sources
(Last.fm, Spotify, etc.) on next access.

Examples:
  # Refresh artist by ID
  navidrome refresh --id "xxxxx"

  # Refresh artist by name (partial match)
  navidrome refresh --name "周杰伦"

  # Clear all external info and refresh
  navidrome refresh --id "xxxxx" --clear-all

  # Only clear images
  navidrome refresh --name "Taylor Swift" --clear-images

  # Refresh artist and their albums
  navidrome refresh --id "xxxxx" --albums`,
	Run: func(cmd *cobra.Command, args []string) {
		runRefreshArtist()
	},
}

func runRefreshArtist() {
	if artistID == "" && artistName == "" {
		fmt.Println("Error: Either --id or --name must be specified")
		os.Exit(1)
	}

	ctx := context.Background()
	defer db.Init(ctx)()

	ds := CreateDataStore()

	var artists model.Artists
	var err error

	if artistID != "" {
		// Get artist by ID
		artist, err := ds.Artist(ctx).Get(artistID)
		if err != nil {
			log.Error("Artist not found", "id", artistID, err)
			fmt.Printf("Error: Artist with ID '%s' not found\n", artistID)
			os.Exit(1)
		}
		artists = model.Artists{*artist}
	} else {
		// Search artist by name
		artists, err = ds.Artist(ctx).GetAll(model.QueryOptions{
			Filters: Like{"artist.name": "%" + artistName + "%"},
		})
		if err != nil {
			log.Error("Error searching artists", "name", artistName, err)
			fmt.Printf("Error searching for artists: %v\n", err)
			os.Exit(1)
		}
		if len(artists) == 0 {
			fmt.Printf("No artists found matching '%s'\n", artistName)
			os.Exit(1)
		}
	}

	fmt.Printf("Found %d artist(s) to refresh:\n", len(artists))
	for _, a := range artists {
		fmt.Printf("  - %s (ID: %s)\n", a.Name, a.ID)
	}
	fmt.Println()

	// Clear and refresh each artist
	for _, artist := range artists {
		err := clearArtistExternalInfo(ctx, ds, &artist)
		if err != nil {
			log.Error("Error clearing artist info", "artist", artist.Name, err)
			fmt.Printf("Error clearing info for '%s': %v\n", artist.Name, err)
			continue
		}
		fmt.Printf("✓ Cleared external info for artist: %s\n", artist.Name)

		// Also refresh albums if requested
		if refreshAlbums {
			albums, err := ds.Album(ctx).GetAll(model.QueryOptions{
				Filters: Eq{"album_artist_id": artist.ID},
			})
			if err != nil {
				log.Error("Error getting albums", "artist", artist.Name, err)
				continue
			}
			for _, album := range albums {
				err := clearAlbumExternalInfo(ctx, ds, &album)
				if err != nil {
					log.Error("Error clearing album info", "album", album.Name, err)
					continue
				}
				fmt.Printf("  ✓ Cleared external info for album: %s\n", album.Name)
			}
		}
	}

	fmt.Println()
	fmt.Println("Done! External info will be re-fetched on next access.")
	fmt.Println("Note: You may also need to clear the artwork cache in the UI or restart the server.")
}

func clearArtistExternalInfo(ctx context.Context, ds model.DataStore, artist *model.Artist) error {
	// Clear external_info_updated_at to force refresh
	artist.ExternalInfoUpdatedAt = nil

	if clearAll || clearImages {
		artist.SmallImageUrl = ""
		artist.MediumImageUrl = ""
		artist.LargeImageUrl = ""
	}

	if clearAll || clearBio {
		artist.Biography = ""
	}

	if clearAll || clearSimilar {
		artist.SimilarArtists = nil
	}

	if clearAll {
		artist.ExternalUrl = ""
	}

	if err := ds.Artist(ctx).UpdateExternalInfo(artist); err != nil {
		return err
	}

	// Since v0.64.0 the artwork pipeline caches resolved images on its own and no longer
	// reads these legacy URL fields, so clearing them alone no longer re-fetches the
	// displayed image. Drop the cached artwork and re-queue it at Bump priority.
	if clearAll || clearImages {
		if err := artwork.Refresh(ctx, ds, model.KindArtistArtwork, artist.ID); err != nil {
			log.Warn("Could not re-queue artist artwork for refresh", "artist", artist.Name, "id", artist.ID, err)
		}
	}

	return nil
}

func clearAlbumExternalInfo(ctx context.Context, ds model.DataStore, album *model.Album) error {
	// Clear external_info_updated_at to force refresh
	album.ExternalInfoUpdatedAt = nil

	if clearAll || clearImages {
		album.SmallImageUrl = ""
		album.MediumImageUrl = ""
		album.LargeImageUrl = ""
	}

	if clearAll {
		album.Description = ""
		album.ExternalUrl = ""
	}

	if err := ds.Album(ctx).UpdateExternalInfo(album); err != nil {
		return err
	}

	if clearAll || clearImages {
		if err := artwork.Refresh(ctx, ds, model.KindAlbumArtwork, album.ID); err != nil {
			log.Warn("Could not re-queue album artwork for refresh", "album", album.Name, "id", album.ID, err)
		}
	}

	return nil
}
