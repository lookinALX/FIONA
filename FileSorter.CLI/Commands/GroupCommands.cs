using CommandLine;
using FileSorter.Core.Helpers;
using FileSorter.Core.Models;
using FileSorter.Core.Services;

namespace FileSorter.CLI.Commands;

public class GroupCommands
{
    private readonly IGroupingService _groupingService;
    private readonly IRollbackService _rollbackService;

    public GroupCommands(IGroupingService groupingService, IRollbackService rollbackService)
    {
        _groupingService = groupingService ?? throw new ArgumentNullException(nameof(groupingService));
        _rollbackService = rollbackService ?? throw new ArgumentNullException(nameof(rollbackService));
    }
    
    public async Task<int> ExecuteAsync(GroupOptions options)
    {
        try
        {
            Console.WriteLine($"Grouping files in: {options.SourceDirectory}");
            Console.WriteLine($"Group by: {options.GroupBy}");
            
            if (!options.GroupWith.Equals("none"))
            {
                Console.WriteLine($"Group second stage by: {options.GroupWith}");
            }
            
            Console.WriteLine($"Conflict strategy: {options.ConflictStrategy}");
            
            if (options.CreateBackup)
            {
                Console.WriteLine($"Backup enabled with suffix: {options.BackupSuffix}");
            }
            
            if (options.HowToHandle.Equals("copy"))
            {
                Console.WriteLine("[RUN TYPE] - Files will be copied (not moved)");
            }
            
            if (options.DryRun)
            {
                Console.WriteLine("[DRY RUN] - No files will be actually moved/copied");
                return await PerformDryRun(options);
            }
            
            var groupingProfile = ParseGroupingProfile(options);

            var result = await _groupingService.GroupFilesAsync(options.GetEffectiveSourceDirectory(), groupingProfile);

            if (result.Success)
            {
                Console.WriteLine($"Operation completed successfully!");
                Console.WriteLine($"Processed {result.ProcessedFiles} files");
                
                if (result.SkippedFiles > 0)
                {
                    Console.WriteLine($"⏭ Skipped {result.SkippedFiles} files due to conflicts");
                }
                
                if (result.FailedFiles > 0)
                {
                    Console.WriteLine($"Failed to process {result.FailedFiles} files");
                }
                
                Console.WriteLine($"⏱ Duration: {result.Duration.TotalSeconds:F2} seconds");
                
                if (result.Errors.Any())
                {
                    Console.WriteLine("\n⚠ Warnings/Errors:");
                    foreach (var error in result.Errors)
                    {
                        Console.WriteLine($"   {error}");
                    }
                }
                
                // Show rollback option
                Console.WriteLine($"\nTo rollback this operation, run: rollback");
            }
            else
            {
                Console.WriteLine("Operation failed!");
                foreach (var error in result.Errors)
                {
                    Console.WriteLine($"   {error}");
                }
                return 1;
            }
            
            return 0;
        }
        catch (Exception ex)
        {
            Console.WriteLine($"Error: {ex.Message}");
            return 1;
        }
    }

    private async Task<int> PerformDryRun(GroupOptions options)
    {
        try
        {
            Console.WriteLine("Analyzing files for dry run...");
            
            var groupingProfile = ParseGroupingProfile(options);
            
            // For dry run, we'll use Copy operation and then analyze what would happen
            var tempProfile = new GroupingProfile
            {
                PrimaryCriteria = groupingProfile.PrimaryCriteria,
                SecondaryCriteria = groupingProfile.SecondaryCriteria,
                DatePrimaryGroupingOption = groupingProfile.DatePrimaryGroupingOption,
                DateSecondaryGroupingOption = groupingProfile.DateSecondaryGroupingOption,
                FileOperationType = FileOperationType.Copy,
                ConflictHandling = new ConflictHandling { Strategy = ConflictResolutionStrategy.Skip }
            };

            var files = await _groupingService.GetFilesToGroupAsync(options.GetEffectiveSourceDirectory());
            var fileList = files.ToList();
            
            Console.WriteLine($"\nDry Run Results:");
            Console.WriteLine($"   Total files found: {fileList.Count}");
            
            // Group files and show what would happen
            var groups = GroupFilesByCriteria(fileList, groupingProfile.PrimaryCriteria, groupingProfile.DatePrimaryGroupingOption);
            
            Console.WriteLine($"   Would create {groups.Count} primary groups:");
            
            foreach (var group in groups)
            {
                Console.WriteLine($"      📁 {group.Key}: {group.Value.Count} files");
                
                // Check for conflicts
                var destinationDir = Path.Combine(options.GetEffectiveSourceDirectory(), group.Key);
                var conflicts = 0;
                
                foreach (var file in group.Value)
                {
                    var destPath = Path.Combine(destinationDir, file.Name);
                    if (File.Exists(destPath))
                    {
                        conflicts++;
                    }
                }
                
                if (conflicts > 0)
                {
                    Console.WriteLine($"         ⚠  {conflicts} potential conflicts");
                }
            }
            
            Console.WriteLine($"\n💡 This was a dry run. Add --dry-run=false to execute the operation.");
            
            return 0;
        }
        catch (Exception ex)
        {
            Console.WriteLine($"Dry run failed: {ex.Message}");
            return 1;
        }
    }

    private static Dictionary<string, List<FileItem>> GroupFilesByCriteria(
        List<FileItem> files, GroupingCriteria criteria, DateGroupingOption option)
    {
        // This is a simplified version for dry run - you might want to extract this to a shared utility
        return criteria switch
        {
            GroupingCriteria.Extension => files.GroupBy(f => f.Extension[1..]).ToDictionary(g => g.Key, g => g.ToList()),
            GroupingCriteria.FileCategory => files.GroupBy(f => f.Category.ToString().ToLower()).ToDictionary(g => g.Key, g => g.ToList()),
            _ => files.GroupBy(f => "misc").ToDictionary(g => g.Key, g => g.ToList())
        };
    }

    private static GroupingProfile ParseGroupingProfile(GroupOptions options)
    {
        var primaryGroupingCriteria = options.GroupBy.ParseGroupingCriteria();
        var secondaryGroupingCriteria = options.GroupWith.ParseGroupingCriteria();
        var primaryDateGroupingOption = options.DateGroupingPrimary.ParseGroupingOption();
        var secondaryDateGroupingOption = options.DateGroupingSecondary.ParseGroupingOption();
        var fileOperationType = options.HowToHandle.ParseFileOperationType();
        var conflictHandling = options.ConflictStrategy.ParseConflictHandling(options.CreateBackup, options.BackupSuffix);
        
        return new GroupingProfile
        {
            PrimaryCriteria = primaryGroupingCriteria,
            SecondaryCriteria = secondaryGroupingCriteria,
            FileOperationType = fileOperationType,
            DatePrimaryGroupingOption = primaryDateGroupingOption,
            DateSecondaryGroupingOption = secondaryDateGroupingOption,
            ConflictHandling = conflictHandling
        };
    }
}

[Verb("rollback", HelpText = "Rollback the last grouping operation")]
public class RollbackOptions
{
    [Option('f', "force", Default = false, HelpText = "Force rollback without confirmation")]
    public bool Force { get; set; }
}

public class RollbackCommands
{
    private readonly IRollbackService _rollbackService;

    public RollbackCommands(IRollbackService rollbackService)
    {
        _rollbackService = rollbackService ?? throw new ArgumentNullException(nameof(rollbackService));
    }

    public async Task<int> ExecuteAsync(RollbackOptions options)
    {
        try
        {
            var history = _rollbackService.GetOperationHistory();
            
            if (!history.Any())
            {
                Console.WriteLine("ℹNo operations to rollback.");
                return 0;
            }

            Console.WriteLine($"Found {history.Count} operations to rollback:");
            
            foreach (var op in history.Take(5))
            {
                Console.WriteLine($"   {op.OperationType}: {op.SourcePath} -> {op.DestinationPath}");
            }
            
            if (history.Count > 5)
            {
                Console.WriteLine($"   ... and {history.Count - 5} more operations");
            }

            if (!options.Force)
            {
                Console.Write("\nAre you sure you want to rollback these operations? (y/N): ");
                var response = Console.ReadKey();
                Console.WriteLine();
                
                if (response.KeyChar != 'y' && response.KeyChar != 'Y')
                {
                    Console.WriteLine("Rollback cancelled.");
                    return 0;
                }
            }

            Console.WriteLine("Starting rollback...");
            
            var result = await _rollbackService.RollbackAsync();
            
            if (result.Success)
            {
                Console.WriteLine($"Rollback completed successfully!");
                Console.WriteLine($"   {result.SuccessfulRollbacks} operations rolled back");
                Console.WriteLine($"   Duration: {result.Duration.TotalSeconds:F2} seconds");
            }
            else
            {
                Console.WriteLine($"⚠  Rollback partially completed:");
                Console.WriteLine($"   {result.SuccessfulRollbacks} successful, {result.FailedRollbacks} failed");
                
                if (result.Errors.Any())
                {
                    Console.WriteLine("Errors:");
                    foreach (var error in result.Errors)
                    {
                        Console.WriteLine($"   {error}");
                    }
                }
            }
            
            return result.Success ? 0 : 1;
        }
        catch (Exception ex)
        {
            Console.WriteLine($"Rollback failed: {ex.Message}");
            return 1;
        }
    }
}