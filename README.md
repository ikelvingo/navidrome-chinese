# Navidrome [China Special Edition] 

## Provide scrobbling artists and albums bio from netease.

## #~~You should use it with [navichina](https://github.com/TooAndy/navichina)~~ 

# Thanks for [TooAndy](https://github.com/TooAndy)'s great work.

# #1139840: Remove navichina dependency in navidrome-chinese.

Input new 'netease' agent for scrobbling artists, albums, similar songs, 
and artist popular songs. 

- Note1: Similar artists functionality not supported.

- Note2: Configuration: Set the ND_AGENTS environment variable to 'netease' to activate the NetEase scrobbling agent.

	```yaml
	# docker compose modify
	  environment:
	    - ND_AGENTS=netease #,deezer,lastfm,listenbrainz
	```

-----

>  [!IMPORTANT]
>
>  **Added the forced refresh Artist data function, providing the following features:**



## How to use

```bash
# Refresh via artist ID
 sudo docker exec -it navidrome refresh --id "xxxxx"

# Refresh via artist name (supports fuzzy matching)
 sudo docker exec -it navidrome refresh --name "Taylor Swift"

# Clear all external information and refresh
 sudo docker exec -it navidrome refresh --id "xxxxx" --clear-all

# Clear only the artist's image URLs
 sudo docker exec -it navidrome refresh --name "Taylor Swift" --clear-images

# Refresh all albums of the artist simultaneously
 sudo docker exec -it navidrome refresh --id "xxxxx" --albums --clear-all
```

## Available parameters

| Parameters        | **Instructions**                            |
| ----------------- | ------------------------------------------- |
| `--id`            | clear artist ID                             |
| `--name`          | clear artist name (supports fuzzy matching) |
| `--clear-images`  | clear image URLs                            |
| `--clear-bio`     | clear artist bio                            |
| `--clear-similar` | clear similar artists                       |
| `--clear-all`     | clear all external infomation               |
| `--albums`        | clear all artist’s albums                   |

After clearing, the next time you visit the artist's page, information will be fetched again from external sources (Last.fm, NetEase Cloud Music, etc.).

-----



