using FileSorter.Core.Models;
using FileSorter.Core.Services;

namespace Tests;

public class GroupingServiceTest
{
    private string _fileDirectoryTest = @"F:\projects\FIONA\Tests\";
    private string _fileDirectoryWithFiles = @"F:\projects\FIONA\Tests\testDir\";
    private GroupingService _groupingService = new GroupingService();
    
    [SetUp]
    public void Setup()
    {
        
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
        Assert.That(outputWithFiles.Count, Is.EqualTo(4));
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

        var service = new GroupingService();
        
        // Act
        var result = await service.GroupFilesAsync(_fileDirectoryWithFiles, profile);
        
        // Assert
        Assert.That(result, Is.Not.Null);
        Assert.That(result.Success, Is.True);
        Assert.That(result.DestinationPathList.Count, Is.EqualTo(4));
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
        
        var service = new GroupingService();
        
        // Act
        var result = await service.GroupFilesAsync(_fileDirectoryWithFiles, profile);
        
        // TODO: Finist assert
        // Assert
        Assert.That(result, Is.Not.Null);
        
        // Cleanup
    }
}