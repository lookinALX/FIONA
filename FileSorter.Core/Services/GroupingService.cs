using FileSorter.Core.Helpers;
using FileSorter.Core.Models;

namespace FileSorter.Core.Services;

public sealed class GroupingService : IGroupingService
{
    public async Task<GroupingResult> GroupFilesAsync(string directoryPath, GroupingProfile profile, CancellationToken cancellationToken = default)
    {
        var result = new GroupingResult();
        
        ValidateDirectory(directoryPath);
        
        if (profile.PrimaryCriteria is GroupingCriteria.None)
        {
            throw new Exception("The primary criteria has not been set.");
        }
        
        var allFiles = await GetFilesToGroupAsync(directoryPath, cancellationToken);

        var fileGroupsPrime = GroupFilesByCriteria(
            allFiles, profile.PrimaryCriteria, profile.DatePrimaryGroupingOption);

        foreach (var group in fileGroupsPrime)
        {
            await ProcessFileGroup(directoryPath, group.Key, group.Value, profile, result, cancellationToken);
        }

        if (profile.SecondaryCriteria is not GroupingCriteria.None)
        {
            throw new NotSupportedException("The secondary criteria has not been set.");
        }
        
        result.Success = true;
        
        return result;
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
    
    private static Dictionary<string, List<FileItem>> GroupFilesByCriteria(
        IEnumerable<FileItem> files, GroupingCriteria criteria, DateGroupingOption option)
    {
        var fileItems = files.ToList();
        return criteria switch
        {
            GroupingCriteria.Extension => GroupFilesByExtension(fileItems),
            GroupingCriteria.ModificationDate => GroupFilesByModificationDate(fileItems, option),
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
    
    private static Dictionary<string, List<FileItem>> GroupFilesByModificationDate(
        List<FileItem> files, DateGroupingOption option)
    {
        return option switch
        {
            DateGroupingOption.Year => files
                .GroupBy(file => file.LastModifiedDate.Year.ToString())
                .ToDictionary(group => group.Key, group => group.ToList()),

            DateGroupingOption.Month => files
                .GroupBy(file => file.LastModifiedDate.ToString("MMMM"))
                .ToDictionary(group => group.Key, group => group.ToList()),

            DateGroupingOption.YearMonth => files
                .GroupBy(file => $"{file.LastModifiedDate:yyyy-MM}")
                .ToDictionary(group => group.Key, group => group.ToList()),

            DateGroupingOption.None => throw new ArgumentException("The grouping option has not been set."),
            _ => throw new NotImplementedException($"Unsupported DateGroupingOption: {option}")
        };
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
    
    private static Task<GroupingResult> ProcessFileGroup(
        string directoryPath,
        string groupKey, 
        List<FileItem> groupValues, 
        GroupingProfile profile, 
        GroupingResult result,
        CancellationToken cancellationToken = default)
    {
        var processedFiles = 0;
        var destinationDirectoryPath = Path.Combine(directoryPath, groupKey[1..]);
        Directory.CreateDirectory(destinationDirectoryPath);
        
        result.DestinationPathList.Add(destinationDirectoryPath);
        
        switch (profile.FileOperationType)
        {
            case FileOperationType.Move:
                foreach (var file in groupValues)
                {
                    File.Move(file.FullPath, Path.Combine(destinationDirectoryPath, file.Name));
                    processedFiles++;
                }
                break;
            case FileOperationType.Copy:
                foreach (var file in groupValues)
                {
                    File.Copy(file.FullPath, Path.Combine(destinationDirectoryPath, file.Name));
                    processedFiles++;
                }
                break;
            default:
                throw new ArgumentOutOfRangeException(nameof(profile.FileOperationType), "Unsupported file operation type.");
        }
        
        result.ProcessedFiles = processedFiles;
        
        return Task.FromResult(result);
    }
}