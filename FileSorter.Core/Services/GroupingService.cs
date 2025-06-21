using FileSorter.Core.Models;

namespace FileSorter.Core.Services;

public sealed class GroupingService : IGroupingService
{
    public async Task<GroupingResult> SortFilesAsync(string directoryPath, GroupingProfile profile, CancellationToken cancellationToken = default)
    {
        ValidateDirectory(directoryPath);

        if (profile.PrimaryCriteria is GroupingCriteria.None)
        {
            throw new Exception("The primary criteria has not been set.");
        }
        
        var allFiles = await GetFilesToGroupAsync(directoryPath, cancellationToken);

        var fileGroupsPrime = GroupFilesByCriteria(allFiles, profile.PrimaryCriteria);

        foreach (var group in fileGroupsPrime)
        {
            await ProcessFileGroup(directoryPath, group.Key, group.Value, profile, cancellationToken);
        }

        if (profile.SecondaryCriteria is not GroupingCriteria.None)
        {

        }
        
        throw new NotImplementedException();
    }

    private void ValidateDirectory(string directoryPath)
    {
        if (!Directory.Exists(directoryPath))
        {
            throw new DirectoryNotFoundException(directoryPath);
        }
    }

    public Task<IEnumerable<FileItem>> GetFilesToGroupAsync(string directoryPath,
        CancellationToken cancellationToken = default)
    {
        var files = Directory.GetFiles(directoryPath);
        var allFiles = files.Select(file => new FileInfo(file)).
            Select(fileInfo => new FileItem(fileInfo)).ToList();
        return Task.FromResult<IEnumerable<FileItem>>(allFiles);
    }
    
    private Dictionary<string, List<FileItem>> GroupFilesByCriteria(
        IEnumerable<FileItem> files, GroupingCriteria criteria)
    {
        var fileItems = files.ToList();
        return criteria switch
        {
            GroupingCriteria.Extension => GroupFilesByExtension(fileItems),
            GroupingCriteria.ModificationDate => GroupFilesByModificationDate(fileItems),
            GroupingCriteria.CreationDate => GroupFilesByCreationDate(fileItems),
            GroupingCriteria.OldestDate => GroupFilesByOldestDate(fileItems),
            GroupingCriteria.FileCategory => GroupFilesByEFileCategory(fileItems),
            GroupingCriteria.None => throw new Exception("The criteria has not been set!"),
            _ => throw new NotImplementedException()
        };
    }

    private static Dictionary<string, List<FileItem>> GroupFilesByExtension(List<FileItem> files)
    {
        var result = files.
            GroupBy(file => file.Extension).
            ToDictionary(group => group.Key, group => group.ToList());
        return result;
    }
    
    private static Dictionary<string, List<FileItem>> GroupFilesByModificationDate(List<FileItem> files)
    {
        throw new NotImplementedException();
    }
    
    private static Dictionary<string, List<FileItem>> GroupFilesByCreationDate(List<FileItem> files)
    {
        throw new NotImplementedException();
    }

    private static Dictionary<string, List<FileItem>> GroupFilesByOldestDate(List<FileItem> files)
    {
        throw new NotImplementedException();
    }

    private static Dictionary<string, List<FileItem>> GroupFilesByEFileCategory(IEnumerable<FileItem> files)
    {
        throw new NotImplementedException();
    }
    
    private static Task ProcessFileGroup(
        string directoryPath,
        string groupKey, 
        List<FileItem> groupValues, 
        GroupingProfile profile, 
        CancellationToken cancellationToken = default)
    {
        var destinationDirectoryPath = Path.Combine(directoryPath, groupKey);
        Directory.CreateDirectory(destinationDirectoryPath);
        switch (profile.FileOperationType)
        {
            case FileOperationType.Move:
                foreach (var file in groupValues)
                {
                    File.Move(file.FullPath, destinationDirectoryPath);
                }
                break;
            case FileOperationType.Copy:
                foreach (var file in groupValues)
                {
                    File.Copy(file.FullPath, destinationDirectoryPath);
                }
                break;
            default:
                throw new ArgumentOutOfRangeException();
        }
        return Task.CompletedTask;
    }
}