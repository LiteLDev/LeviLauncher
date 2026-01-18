import { ListDir } from "bindings/github.com/liteldev/LeviLauncher/minecraft";

export type DirEntry = { name: string; path: string };

export async function listDirectories(path: string): Promise<DirEntry[]> {
  try {
    const list = await ListDir(path);
    return (list || [])
      .filter((e: any) => e.isDir)
      .map((e: any) => ({ name: e.name, path: e.path }));
  } catch {
    return [];
  }
}

export async function countDirectories(path: string): Promise<number> {
  try {
    const list = await ListDir(path);
    return (list || []).filter((e: any) => e.isDir).length;
  } catch {
    return 0;
  }
}
