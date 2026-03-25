#  Navidrome [China Special Edition] 

``` bash
docker pull ikelvingo/navidrome-chinese:lastest
```

## Provide scrobbling artists and albums bio from netease.

## #~~You should use it with [navichina](https://github.com/TooAndy/navichina)~~ 

# Thanks for TooAndy's great work.

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
>  **引入OpenCC，终于统一了Navidrome中文繁简体搜索**
>
>  - 实现无论搜索“周杰伦”还是“周杰倫”，系统会将“周杰伦”+“周杰倫”的所有搜索结果返回。
>  - 本次更新的搜索功能，无论web端还是subsonic api接口均生效。
>  	**关于部分OpenSubsonic客户端Artist繁简搜索限制的说明**

**问题发现**：经测试，部分使用OpenSubsonic API的客户端（如音流、Chora）在Artist搜索时，繁简转换功能可能无法生效。

**原因分析**：这些客户端通过`albumArtists`接口获取艺术家列表后，**在本地执行搜索**，而非调用Navidrome的`search`接口。因此：

- ✅ Song/Album搜索：繁简转换正常（调用服务端接口）
- ❌ Artist搜索：繁简转换可能失效（本地搜索）

**解决方案**：

1. 如需在这些客户端中实现Artist繁简搜索，建议联系客户端开发者适配
2. **推荐客户端**：[Amcfy Music/箭头音乐](https://cn.amcfy.com/) - 完全兼容本次搜索优化

**技术说明**：此限制源于客户端实现逻辑差异，不影响Navidrome服务端功能完整性。

-----

>  [!IMPORTANT]
>
>  **Added the forced refresh Artist data function, providing the following features:**

##  How to use

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



