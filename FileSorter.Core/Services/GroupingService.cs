using System.Globalization;
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

        var fileGroups = GroupFilesByCriteria(
            allFiles, profile.PrimaryCriteria, profile.DatePrimaryGroupingOption);

        foreach (var group in fileGroups)
        {
            await ProcessFileGroup(directoryPath, group.Key, group.Value, profile, result, "primary", cancellationToken);
        }

        if (profile.SecondaryCriteria is not GroupingCriteria.None)
        {
            foreach (var dirPath in result.DestinationPathDict["primary"])
            {
                allFiles = await GetFilesToGroupAsync(dirPath, cancellationToken);
                fileGroups = GroupFilesByCriteria(allFiles, profile.SecondaryCriteria, profile.DateSecondaryGroupingOption);
                
                foreach (var group in fileGroups)
                {
                    await ProcessFileGroup(dirPath, group.Key, group.Value, profile, result, "secondary", cancellationToken);
                }
            }
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
            GroupingCriteria.ModificationDate => GroupFilesByDate(fileItems, criteria, option),
            GroupingCriteria.CreationDate => GroupFilesByDate(fileItems, criteria, option),
            GroupingCriteria.OldestDate => GroupFilesByOldestDate(fileItems, option),
            GroupingCriteria.FileCategory => GroupFilesByEFileCategory(fileItems),
            GroupingCriteria.None => throw new Exception("The criteria has not been set!"),
            _ => throw new NotImplementedException()
        };
    }

    private static Dictionary<string, List<FileItem>> GroupFilesByExtension(List<FileItem> files)
    {
        var result = files.
            GroupBy(file => file.Extension[1..]).
            ToDictionary(group => group.Key, group => group.ToList());
        return result;
    }

    private static Dictionary<string, List<FileItem>> GroupFilesByDate(
        List<FileItem> files, GroupingCriteria criteria, DateGroupingOption option)
    {
        return files
            .GroupBy(file =>
            {
                DateTime date = criteria switch
                {
                    GroupingCriteria.CreationDate => file.CreationDate,
                    GroupingCriteria.ModificationDate => file.LastModifiedDate,
                    _ => throw new ArgumentOutOfRangeException(nameof(criteria), "Invalid grouping criteria")
                };

                return option switch
                {
                    DateGroupingOption.Year => date.ToString("yyyy"),
                    DateGroupingOption.Month => date.ToString("MMMM", CultureInfo.CreateSpecificCulture("en-US")),
                    DateGroupingOption.YearMonth => date.ToString("yyyy-MM"),
                    DateGroupingOption.None => throw new ArgumentException("The grouping option has not been set."),
                    _ => throw new NotImplementedException($"Unsupported DateGroupingOption: {option}")
                };
            })
            .ToDictionary(group => group.Key, group => group.ToList());
    }

    private static Dictionary<string, List<FileItem>> GroupFilesByOldestDate(
        List<FileItem> files, DateGroupingOption option)
    {
        var result = new Dictionary<string, List<FileItem>>();
        var keyForOption = "";
        
        foreach (var fileItem in files)
        {
            keyForOption = GetKeyForOption(option
                , fileItem.CreationDate <= fileItem.LastModifiedDate ? fileItem.CreationDate : fileItem.LastModifiedDate);

            if (result.TryGetValue(keyForOption, out var fileItems))
            {
                fileItems.Add(fileItem);
            }
            else 
            {
                result[keyForOption] = new List<FileItem> { fileItem };
            }
        }
        return result;
    }

    private static string GetKeyForOption(DateGroupingOption option, DateTime date)
    {
        return option switch
        {
            DateGroupingOption.Year => date.ToString("yyyy"),
            
            DateGroupingOption.Month => date.ToString("MMMM", CultureInfo.CreateSpecificCulture("en-US")),
            
            DateGroupingOption.YearMonth => $"{date:yyyy-MM}",
            
            DateGroupingOption.None => throw new ArgumentException("The grouping option has not been set."),
            
            _ => throw new NotImplementedException($"Unsupported DateGroupingOption: {option}")
        };
    }
    
    private static Dictionary<string, List<FileItem>> GroupFilesByEFileCategory(IEnumerable<FileItem> files)
    {
        return files.GroupBy(file => GetFileCategory(file.Extension))
            .ToDictionary(group => group.Key, group => group.ToList());
    }
    
    private static string GetFileCategory(string extension)
    {
        return extension.ToLowerInvariant() switch
        {
            ".jpg" or ".jpeg" or ".png" or ".gif" or ".bmp" or ".tiff" or ".webp" or ".svg" => "images",
            ".mp4" or ".avi" or ".mkv" or ".mov" or ".wmv" or ".flv" or ".webm" or ".m4v" => "videos", 
            ".mp3" or ".wav" or ".flac" or ".aac" or ".ogg" or ".wma" or ".m4a" => "audio",
            ".pdf" or ".doc" or ".docx" or ".txt" or ".rtf" or ".odt" or ".pages" => "documents",
            ".xls" or ".xlsx" or ".csv" or ".ods" or ".numbers" => "spreadsheets",
            ".ppt" or ".pptx" or ".odp" or ".key" => "presentations",
            ".zip" or ".rar" or ".7z" or ".tar" or ".gz" or ".bz2" or ".xz" => "archives",
            ".exe" or ".msi" or ".deb" or ".rpm" or ".dmg" or ".pkg" => "applications",
            ".iso" or ".img" or ".vhd" or ".vmdk" => "disk images",
            _ => "other"
        };
    }
    
    private static Task<GroupingResult> ProcessFileGroup(
        string directoryPath,
        string groupKey, 
        List<FileItem> groupValues, 
        GroupingProfile profile, 
        GroupingResult result,
        string pathKey,
        CancellationToken cancellationToken = default)
    {
        var processedFiles = 0;
        var destinationDirectoryPath = Path.Combine(directoryPath, groupKey);
        Directory.CreateDirectory(destinationDirectoryPath);
        
        result.DestinationPathList.Add(destinationDirectoryPath);
        if (result.DestinationPathDict.TryGetValue(pathKey, out var list))
        {
            list.Add(destinationDirectoryPath);
        }
        else
        {
            result.DestinationPathDict[pathKey] = new List<string> { destinationDirectoryPath };
        }
        
        if (pathKey.Equals("secondary", StringComparison.InvariantCultureIgnoreCase))
        {
            foreach (var file in groupValues)
            {
                File.Move(file.FullPath, Path.Combine(destinationDirectoryPath, file.Name));
                processedFiles++;
            }
        }
        else
        {
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
        }
        
        result.ProcessedFiles += processedFiles;
        
        return Task.FromResult(result);
    }
}