
using FileSorter.Core.Helpers;
using FileSorter.Core.Models;

const string directory = @"F:\projects\FIONA\FileSorter.CLI\testDir";

List<FileItem> files = new List<FileItem>();

foreach (var file in Directory.GetFiles(directory))
{
    FileInfo fileInfo = new FileInfo(file);
    files.Add(new FileItem(fileInfo));
}

foreach (var file in files)
{
    Console.WriteLine(file.Name);
}
