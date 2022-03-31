package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/runtime"
)

// SetCacheSize is the administrative command that sets the server's cache size for
// storing previously-compiled service handlers. If you specify a smaller number
// that the current cache size, the next attempt to load a new service into the cache
// will result in discarding the oldest cache entries until the cache is the correct
// size. You must be an admin user with a valid token to perform this command.
func SetCacheSize(c *cli.Context) *errors.EgoError {
	if c.GetParameterCount() == 0 {
		return errors.New(errors.ErrCacheSizeNotSpecified)
	}

	size, err := strconv.Atoi(c.GetParameter(0))
	if !errors.Nil(err) {
		return errors.New(err)
	}

	cacheStatus := defs.CacheResponse{
		Limit: size,
	}

	err = runtime.Exchange(defs.AdminCachesPath, http.MethodPost, &cacheStatus, &cacheStatus, defs.AdminAgent)
	if !errors.Nil(err) {
		return errors.New(err)
	}

	if ui.OutputFormat == ui.TextFormat {
		ui.Say("Server cache size updated")
	} else {
		_ = commandOutput(cacheStatus)
	}

	return nil
}

// FlushServerCaches is the administrative command that directs the server to
// discard any cached compilation units for service code. Subsequent service
// requests require that the service code be reloaded from disk. This is often
// used when making changes to a service, to quickly force the server to pick up
// the changes. You must be an admin user with a valid token to perform this command.
func FlushServerCaches(c *cli.Context) *errors.EgoError {
	cacheStatus := defs.CacheResponse{}

	err := runtime.Exchange(defs.AdminCachesPath, http.MethodDelete, nil, &cacheStatus, defs.AdminAgent)
	if !errors.Nil(err) {
		return err
	}

	switch ui.OutputFormat {
	case ui.JSONIndentedFormat:
		b, _ := json.MarshalIndent(cacheStatus, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

		fmt.Println(string(b))

	case ui.JSONFormat:
		b, _ := json.Marshal(cacheStatus)

		fmt.Println(string(b))

	case ui.TextFormat:
		ui.Say("Server cache emptied")
	}

	return nil
}

// ListServerCahces is the administrative command that displays the information about
// the server's cache of previously-compiled service programs. The current and maximum
// size of the cache, and the endpoints that are cached are listed. You must be an
// admin user with a valid token to perform this command.
func ListServerCaches(c *cli.Context) *errors.EgoError {
	cacheStatus := defs.CacheResponse{}

	err := runtime.Exchange(defs.AdminCachesPath, http.MethodGet, nil, &cacheStatus, defs.AdminAgent)
	if !errors.Nil(err) {
		return err
	}

	if ui.OutputFormat == ui.TextFormat {
		fmt.Printf("Server Cache, hostname %s, ID %s\n", cacheStatus.Hostname, cacheStatus.ID)

		if cacheStatus.Count > 0 {
			fmt.Printf("\n")

			t, _ := tables.New([]string{"URL Path", "Count", "Last Used"})

			for _, v := range cacheStatus.Items {
				_ = t.AddRowItems(v.Name, v.Count, v.LastUsed)
			}

			_ = t.SortRows(0, true)
			_ = t.SetIndent(2)
			t.SetPagination(0, 0)

			t.Print(ui.TextFormat)
			fmt.Printf("\n")
		}

		switch cacheStatus.AssetCount {
		case 0:
			fmt.Printf("  There are no HTML assets cached.\n")

		case 1:
			fmt.Printf("  There is 1 HTML asset in cache, for a total size of %d bytes\n", cacheStatus.AssetSize)

		default:
			fmt.Printf("  There are %d HTML assets in cache, for a total size of %d bytes\n", cacheStatus.AssetCount, cacheStatus.AssetSize)
		}

		switch cacheStatus.Count {
		case 0:
			fmt.Printf("  There are no service items in cache. The maximum cache size is %d items\n", cacheStatus.Limit)

		case 1:
			fmt.Printf("  There is 1 service item in cache. The maximum cache size is %d items\n", cacheStatus.Limit)

		default:
			fmt.Printf("  There are %d service items in cache. The maximum cache size is %d items\n", cacheStatus.Count, cacheStatus.Limit)
		}
	} else {
		_ = commandOutput(cacheStatus)
	}

	return nil
}
