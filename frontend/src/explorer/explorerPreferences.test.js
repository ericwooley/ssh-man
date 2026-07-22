import { describe, expect, test } from 'vitest'
import {
  addFavorite,
  readFavoritePaths,
  removeFavorite,
} from './explorerPreferences'

describe('explorer preferences', () => {
  test('normalizes, deduplicates, and sorts favorite paths', () => {
    expect(addFavorite(['/srv/www', '/opt/logs'], '/srv/www/')).toEqual(['/opt/logs', '/srv/www'])
    expect(addFavorite(['/srv/www'], '/srv/api')).toEqual(['/srv/api', '/srv/www'])
    expect(removeFavorite(['/srv/api', '/srv/www'], '/srv/api/')).toEqual(['/srv/www'])
  })

  test('recovers safely from malformed persisted favorites', () => {
    const storage = { getItem: () => '{bad json' }
    expect(readFavoritePaths(storage, 'server-1')).toEqual([])
  })
})
