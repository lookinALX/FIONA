using System.Globalization;
using FileSorter.Core.Helpers;
using FileSorter.Core.Models;

namespace FileSorter.Core.Services;

public sealed class GroupingService : IGroupingService
{
    private readonly IRollbackService _rollbackService;
    private readonly IConflictResolver _conflictResolver;

    public GroupingService(IRollbackService rollbackService, IConflictResolver conflictResolver)
    {
        _rollbackService = rollbackService ?? throw new ArgumentNullException(nameof(rollbackService));
        _conflictResolver = conflictResolver ?? throw new ArgumentNullException(nameof(conflictResolver));
    }

    public async Task<GroupingResult> GroupFilesAsync(string directoryPath, GroupingProfile profile, CancellationToken cancellationToken = default)
    {
        var result = new GroupingResult();
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();
        
        ValidateDirectory(directoryPath);
        
        if (profile.PrimaryCriteria is GroupingCriteria.None)
        {
            throw new Exception("The primary criteria has not been set.");
        }
        
        // Get all files first to show accurate progress
        var allFiles = await GetFilesToGroupAsync(directoryPath, cancellationToken);
        var filesList = allFiles.ToList();
        
        if (!filesList.Any())
        {
            Console.WriteLine("No files found to process.");
            result.Success = true;
            return result;
        }

        var fileGroups = GroupFilesByCriteria(filesList, profile.PrimaryCriteria, profile.DatePrimaryGroupingOption);
        
        Console.WriteLine($"Found {filesList.Count} files in {fileGroups.Count} groups");
        
        // Primary grouping with progress bar
        using (var progressBar = new ProgressBar(filesList.Count, "Grouping files"))
        {
            var processedCount = 0;
            
            foreach (var group in fileGroups)
            {
                await ProcessFileGroup(directoryPath, group.Key, group.Value, profile, result, "primary", 
                    (processed) => 
                    {
                        processedCount += processed;
                        progressBar.Update(processedCount);
                    }, cancellationToken);
                
                if (cancellationToken.IsCancellationRequested)
                {
                    await RollbackOnCancellation(result);
                    return result;
                }
            }
            
            progressBar.Complete();
        }

        // Secondary grouping if specified
        if (profile.SecondaryCriteria is not GroupingCriteria.None)
        {
            Console.WriteLine("Applying secondary grouping...");
            
            var secondaryFiles = new List<FileItem>();
            foreach (var dirPath in result.DestinationPathDict["primary"])
            {
                var files = await GetFilesToGroupAsync(dirPath, cancellationToken);
                secondaryFiles.AddRange(files);
            }
            
            using (var progressBar = new ProgressBar(secondaryFiles.Count, "Secondary grouping"))
            {
                var processedCount = 0;
                
                foreach (var dirPath in result.DestinationPathDict["primary"])
                {
                    var files = await GetFilesToGroupAsync(dirPath, cancellationToken);
                    var fileGroups2 = GroupFilesByCriteria(files, profile.SecondaryCriteria, profile.DateSecondaryGroupingOption);
                    
                    foreach (var group in fileGroups2)
                    {
                        await ProcessFileGroup(dirPath, group.Key, group.Value, profile, result, "secondary", 
                            (processed) => 
                            {
                                processedCount += processed;
                                progressBar.Update(processedCount);
                            }, cancellationToken);
                        
                        if (cancellationToken.IsCancellationRequested)
                        {
                            await RollbackOnCancellation(result);
                            return result;
                        }
                    }
                }
                
                progressBar.Complete();
            }
        }
        
        stopwatch.Stop();
        result.Duration = stopwatch.Elapsed;
        result.Success = true;
        
        return result;
    }

    private async Task RollbackOnCancellation(GroupingResult result)
    {
        try
        {
            Console.WriteLine("\nOperation cancelled. Starting rollback...");
            using var progressBar = new ProgressBar(1, "Rolling back");
            
            var rollbackResult = await _rollbackService.RollbackAsync();
            progressBar.Complete();
            
            result.Errors.Add($"Operation cancelled. Rollback: {rollbackResult.SuccessfulRollbacks} operations rolled back, {rollbackResult.FailedRollbacks} failed");
            result.Errors.AddRange(rollbackResult.Errors);
        }
        catch (Exception ex)
        {
            result.Errors.Add($"Rollback after cancellation failed: {ex.Message}");
        }
    }
    
    private void ValidateDirectory(string directoryPath)
    {
        if (!Directory.Exists(directoryPath))
        {
            throw new DirectoryNotFoundException(directoryPath);
        }
        
        try
        {
            var testFile = Path.Combine(directoryPath, $"test_write_{Guid.NewGuid()}.tmp");
            File.WriteAllText(testFile, "test");
            File.Delete(testFile);
        }
        catch (UnauthorizedAccessException)
        {
            throw new UnauthorizedAccessException($"No write access to directory: {directoryPath}");
        }
    }

    public async Task<IEnumerable<FileItem>> GetFilesToGroupAsync(string directoryPath,
        CancellationToken cancellationToken = default)
    {
        return await Task.Run(() =>
        {
            var files = Directory.GetFiles(directoryPath);
            return files.Select(file => new FileInfo(file))
                       .Select(fileInfo => new FileItem(fileInfo))
                       .ToList();
        }, cancellationToken);
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
    
    private async Task ProcessFileGroup(
        string directoryPath,
        string groupKey, 
        List<FileItem> groupValues, 
        GroupingProfile profile, 
        GroupingResult result,
        string pathKey,
        Action<int> onProgress,
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

        foreach (var file in groupValues)
        {
            if (cancellationToken.IsCancellationRequested)
                break;

            try
            {
                var destinationPath = Path.Combine(destinationDirectoryPath, file.Name);
                
                var conflictResult = await _conflictResolver.ResolveConflictAsync(
                    file.FullPath, destinationPath, profile.ConflictHandling, cancellationToken);
                
                if (!conflictResult.ShouldProceed)
                {
                    result.SkippedFiles++;
                    processedFiles++;
                    onProgress(1);
                    continue;
                }
                
                var operationRecord = new FileOperationRecord
                {
                    SourcePath = file.FullPath,
                    DestinationPath = conflictResult.FinalDestinationPath,
                    OperationType = pathKey.Equals("secondary", StringComparison.InvariantCultureIgnoreCase) 
                        ? FileOperationType.Move 
                        : profile.FileOperationType,
                    Timestamp = DateTime.Now,
                    BackupPath = conflictResult.BackupPath,
                    ConflictResolution = conflictResult.AppliedStrategy
                };
                
                await ExecuteFileOperation(operationRecord, cancellationToken);
                
                operationRecord.Success = true;
                _rollbackService.RecordOperation(operationRecord);
                
                processedFiles++;
                onProgress(1);
            }
            catch (Exception e)
            {
                result.FailedFiles++;
                result.Errors.Add($"Failed to process {file.Name}: {e.Message}");
                processedFiles++;
                onProgress(1);
            }
        }
        result.ProcessedFiles += processedFiles;
    }

    private static async Task ExecuteFileOperation(FileOperationRecord operationRecord,
        CancellationToken cancellationToken = default)
    {
        switch (operationRecord.OperationType)
        {
            case FileOperationType.Move:
                await Task.Run(() => File.Move(operationRecord.SourcePath, operationRecord.DestinationPath), cancellationToken);
                break;
            case FileOperationType.Copy:
                await Task.Run(() => File.Copy(operationRecord.SourcePath, operationRecord.DestinationPath), cancellationToken);
                break;
            default:
                throw new ArgumentOutOfRangeException(nameof(operationRecord.OperationType), "Unsupported file operation type.");
        }
    }
}