using FileSorter.Core.Helpers;
using FileSorter.Core.Models;
using FileSorter.Core.Services;

namespace FileSorter.CLI.Commands;

public class GroupCommands
{
    private readonly IGroupingService _groupingService;

    public GroupCommands(IGroupingService groupingService)
    {
        _groupingService = groupingService;
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
            
            if (options.HowToHandle.Equals("copy"))
            {
                Console.WriteLine("[RUN TYPE] - No files will be moved");
            }
            
            var groupingProfile = ParseGroupingProfile(options);

            var result = await _groupingService.GroupFilesAsync(options.SourceDirectory, groupingProfile);

            Console.WriteLine($"✅ Processed {result.ProcessedFiles} files");
            
            return 0;
        }
        catch (Exception ex)
        {
            Console.WriteLine($"❌ Error: {ex.Message}");
            return 1;
        }
    }

    private static GroupingProfile ParseGroupingProfile(GroupOptions options)
    {
        var primaryGroupingCriteria = options.GroupBy.ParseGroupingCriteria();
        var secondaryGroupingCriteria = options.GroupWith.ParseGroupingCriteria();
        var primaryDateGroupingOption = options.DateGroupingPrimary.ParseGroupingOption();
        var secondaryDateGroupingOption = options.DateGroupingSecondary.ParseGroupingOption();
        var fileOperationType = options.HowToHandle.ParseFileOperationType();
        
        return new GroupingProfile
        {
            PrimaryCriteria = primaryGroupingCriteria,
            SecondaryCriteria = secondaryGroupingCriteria,
            FileOperationType = fileOperationType,
            DatePrimaryGroupingOption = primaryDateGroupingOption,
            DateSecondaryGroupingOption = secondaryDateGroupingOption
        };
    }
}