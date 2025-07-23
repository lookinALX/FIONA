using Avalonia.Platform;
using FileSorter.Core.Models;
using FileSorter.Core.Services;

namespace Tests;

// TODO: correct tests
using FileSorter.Core.Models;
using FileSorter.Core.Services;
using NUnit.Framework;

public class GroupingServiceTest
{
    private string _fileDirectoryTest = @"F:\projects\FIONA\Tests\";
    private string _fileDirectoryWithFiles = @"F:\projects\FIONA\Tests\testDir\";
    private GroupingService _groupingService;
    private IRollbackService _rollbackService;
    private IConflictResolver _conflictResolver;
    
    [SetUp]
    public void Setup()
    {
        _rollbackService = new RollbackService();
        _conflictResolver = new ConflictResolver();
        _groupingService = new GroupingService(_rollbackService, _conflictResolver);
    }

    [Test]
    public async Task Test_GetFilesToGroupAsync_ExistingDirectory()
    {
        // Arrange
        var fileDirectoryWithoutFiles = Path.Combine(_fileDirectoryTest, "testDirNoFiles");
        Directory.CreateDirectory(fileDirectoryWithoutFiles);
        
        // Act
        var resultWithFiles = await _groupingService.GetFilesToGroupAsync(_fileDirectoryWithFiles);
        var resultWithoutFiles = await _groupingService.GetFilesToGroupAsync(fileDirectoryWithoutFiles);
        
        var outputWithFiles = resultWithFiles.ToList();
        var outputWithoutFiles = resultWithoutFiles.ToList();
        
        // Assert
        Assert.That(outputWithFiles, Is.Not.Null);
        Assert.That(outputWithFiles.Count, Is.EqualTo(5));
        Assert.That(outputWithoutFiles, Is.Not.Null);
        Assert.That(outputWithoutFiles, Is.Empty);
        
        // Cleanup
        Directory.Delete(fileDirectoryWithoutFiles, true);
    }

    [Test]
    public async Task Test_GroupFilesAsync_By_Extension()
    {
        // Arrange
        var profile = new GroupingProfile
        {
            PrimaryCriteria = GroupingCriteria.Extension,
            SecondaryCriteria = GroupingCriteria.None,
            FileOperationType = FileOperationType.Copy
        };
        
        // Act
        var result = await _groupingService.GroupFilesAsync(_fileDirectoryWithFiles, profile);
        
        // Assert
        Assert.That(result, Is.Not.Null);
        Assert.That(result.Success, Is.True);
        Assert.That(result.DestinationPathList.Count, Is.EqualTo(5));
        Assert.That(result.DestinationPathList.Any(dir => Directory.GetFiles(dir).Length > 0), 
            "Expected at least one destination directory to contain files.");
        
        // Cleanup
        foreach (var dir in result.DestinationPathList)
        {
            Directory.Delete(dir, true);
        }
    }

    [Test]
    public async Task Test_GroupFileAsync_By_Oldest_Year_Date()
    {
        // Arrange
        var profile = new GroupingProfile
        {
            PrimaryCriteria = GroupingCriteria.OldestDate,
            SecondaryCriteria = GroupingCriteria.None,
            DatePrimaryGroupingOption = DateGroupingOption.Year,
            FileOperationType = FileOperationType.Copy
        };
        
        // Act
        var result = await _groupingService.GroupFilesAsync(_fileDirectoryWithFiles, profile);
        
        // Assert
        Assert.That(result, Is.Not.Null);
        Assert.That(result.Success, Is.True);
        Assert.That(result.DestinationPathList.Count, Is.EqualTo(2));
        Assert.That(result.DestinationPathList.Any(dir => Directory.GetFiles(dir).Length > 0), 
            "Expected at least one destination directory to contain files.");
        Assert.That(result.DestinationPathList.Any(dir => Path.GetFileName(dir) == "2025"));
        Assert.That(result.DestinationPathList.Any(dir => Path.GetFileName(dir) == "2022"));
        
        // Cleanup
        foreach (var dir in result.DestinationPathList)
        {
            Directory.Delete(dir, true);
        }
    }

    [Test]
    public async Task Test_GroupFileAsync_By_CreationDate_Month()
    {
        // Arrange
        var profile = new GroupingProfile
        {
            PrimaryCriteria = GroupingCriteria.CreationDate,
            SecondaryCriteria = GroupingCriteria.None,
            DatePrimaryGroupingOption = DateGroupingOption.Month,
            FileOperationType = FileOperationType.Copy
        };
        
        // Act
        var result = await _groupingService.GroupFilesAsync(_fileDirectoryWithFiles, profile);
        
        // Assert
        Assert.That(result, Is.Not.Null);
        Assert.That(result.Success, Is.True);
        Assert.That(result.DestinationPathList.Count, Is.EqualTo(1));
        Assert.That(result.DestinationPathList.Any(dir => Path.GetFileName(dir) == "June"));
        
        // Cleanup
        foreach (var dir in result.DestinationPathList)
        {
            Directory.Delete(dir, true);
        }
    }

    [Test]
    public async Task Test_GroupFileAsync_By_CreationDate_YearMonth_Primary_Extension_Secondary()
    {
        // Arrange
        var profile = new GroupingProfile
        {
            PrimaryCriteria = GroupingCriteria.CreationDate,
            SecondaryCriteria = GroupingCriteria.Extension,
            DatePrimaryGroupingOption = DateGroupingOption.YearMonth,
            FileOperationType = FileOperationType.Copy
        };
        
        // Act
        var result = await _groupingService.GroupFilesAsync(_fileDirectoryWithFiles, profile);
        
        // Assert
        Assert.That(result, Is.Not.Null);
        Assert.That(result.Success, Is.True);
        Assert.That(result.DestinationPathDict["primary"].Count, Is.EqualTo(1));
        Assert.That(result.DestinationPathDict["secondary"].Count, Is.EqualTo(5));
        
        // Cleanup
        foreach (var dir in result.DestinationPathDict["primary"])
        {
            Directory.Delete(dir, true);
        }
    }

    [Test]
    public async Task Test_GroupFileAsync_By_FileCategory()
    {
        // Arrange
        var profile = new GroupingProfile
        {
            PrimaryCriteria = GroupingCriteria.FileCategory,
            SecondaryCriteria = GroupingCriteria.None,
            FileOperationType = FileOperationType.Copy
        };
        
        // Act
        var result = await _groupingService.GroupFilesAsync(_fileDirectoryWithFiles, profile);
        
        // Assert
        Assert.That(result, Is.Not.Null);
        Assert.That(result.Success, Is.True);
        Assert.That(result.DestinationPathDict["primary"].Count, Is.EqualTo(2));
        Assert.That(result.DestinationPathDict["primary"].Any(dir => Path.GetFileName(dir) == "documents"));
        Assert.That(result.DestinationPathDict["primary"].Any(dir => Path.GetFileName(dir) == "images"));
        
        // Cleanup
        foreach (var dir in result.DestinationPathDict["primary"])
        {
            Directory.Delete(dir, true);
        }
    }
}